package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

// Custom Validator Instance with all configurations
var validate *validator.Validate

func init() {
	validate = validator.New()
	
	// Register custom type handlers
	validate.RegisterCustomTypeFunc(validateUint64, uint64(0), []uint64{})
	
	// Register custom validations
	validate.RegisterValidation("in", func(fl validator.FieldLevel) bool {
		values := strings.Split(fl.Param(), ",")
		fieldValue := fmt.Sprintf("%v", fl.Field().Interface())
		
		for _, v := range values {
			if v == fieldValue {
				return true
			}
		}
		return false
	})
	
	validate.RegisterValidation("unique", func(fl validator.FieldLevel) bool {
		switch fl.Field().Kind() {
		case reflect.Slice, reflect.Array:
			unique := make(map[string]struct{})
			for i := 0; i < fl.Field().Len(); i++ {
				val := fmt.Sprintf("%v", fl.Field().Index(i).Interface())
				if _, exists := unique[val]; exists {
					return false
				}
				unique[val] = struct{}{}
			}
		}
		return true
	})
}

func validateUint64(field reflect.Value) interface{} {
	if field.CanUint() {
		return field.Uint()
	}
	return nil
}

// ValidationError represents a single field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// RequestParser defines where to get the data from
type RequestParser string

const (
	BodyParser  RequestParser = "body"
	QueryParser RequestParser = "query"
	ParamParser RequestParser = "param"
)

// ValidationConfig for each field
type ValidationConfig struct {
	FieldName    string
	Parser       RequestParser
	Required     bool
	Default      string
	ValidateTags string
}

// GenericRequest represents all possible field types
type GenericRequest struct {
	// String fields
	Name     string   `json:"name" query:"name" param:"name"`
	Email    string   `json:"email" query:"email"`
	
	// Slice fields
	Tags     []string `json:"tags" query:"tags"`
	IDs      []uint64 `json:"ids" query:"ids"`
	
	// Numeric fields
	UserID   uint64   `json:"user_id" param:"user_id"`
	Rating   int      `json:"rating" query:"rating"`
	
	// Internal
	validationGroup string `json:"-"` // Used for conditional validation
}

// ValidateRequest validates based on config
func ValidateRequest(configs []ValidationConfig, group string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		req := &GenericRequest{validationGroup: group}
		errors := make([]ValidationError, 0)
		
		// Parse all configured fields
		for _, cfg := range configs {
			field := reflect.ValueOf(req).Elem().FieldByName(cfg.FieldName)
			if !field.IsValid() {
				continue
			}
			
			// Get value based on parser
			rawValue, found := getRawValue(r, cfg)
			if !found {
				if cfg.Required {
					errors = append(errors, ValidationError{
						Field:   cfg.FieldName,
						Message: "This field is required",
					})
				}
				continue
			}
			
			// Set the value with proper type conversion
			if err := setFieldValue(field, rawValue, cfg); err != nil {
				errors = append(errors, ValidationError{
					Field:   cfg.FieldName,
					Message: err.Error(),
					Value:   rawValue,
				})
			}
		}
		
		// If parsing errors exist, return early
		if len(errors) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: "Invalid request data",
				Errors:  errors,
			})
			return
		}
		
		// Validate the struct
		if err := validate.Struct(req); err != nil {
			validationErrors := err.(validator.ValidationErrors)
			verrs := make([]ValidationError, len(validationErrors))
			
			for i, e := range validationErrors {
				verrs[i] = ValidationError{
					Field:   e.StructField(),
					Message: getValidationMessage(e),
					Value:   fmt.Sprintf("%v", e.Value()),
				}
			}
			
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(ErrorResponse{
				Code:    http.StatusUnprocessableEntity,
				Message: "Validation failed",
				Errors:  verrs,
			})
			return
		}
		
		// Validation passed - call next handler
		// nextHandler(w, r, req)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "valid",
			"data":   req,
		})
	}
}

func getRawValue(r *http.Request, cfg ValidationConfig) (string, bool) {
	switch cfg.Parser {
	case BodyParser:
		// Body parsing handled by json decoder later
		return "", false
	case QueryParser:
		return r.URL.Query().Get(cfg.FieldName), r.URL.Query().Has(cfg.FieldName)
	case ParamParser:
		vars := mux.Vars(r)
		val, exists := vars[cfg.FieldName]
		return val, exists
	default:
		return "", false
	}
}

func setFieldValue(field reflect.Value, rawValue string, cfg ValidationConfig) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(rawValue)
		
	case reflect.Int, reflect.Int64:
		val, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a valid integer")
		}
		field.SetInt(val)
		
	case reflect.Uint, reflect.Uint64:
		val, err := strconv.ParseUint(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a positive integer")
		}
		field.SetUint(val)
		
	case reflect.Slice:
		values := strings.Split(rawValue, ",")
		switch field.Type().Elem().Kind() {
		case reflect.String:
			slice := make([]string, len(values))
			for i, v := range values {
				slice[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(slice))
			
		case reflect.Uint64:
			slice := make([]uint64, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
				if err != nil {
					return fmt.Errorf("element %d: must be positive integer", i+1)
				}
				slice[i] = val
			}
			field.Set(reflect.ValueOf(slice))
			
		default:
			return fmt.Errorf("unsupported slice type")
		}
		
	default:
		return fmt.Errorf("unsupported field type")
	}
	return nil
}

func getValidationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "min":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Minimum %s characters required", e.Param())
		} else if e.Kind() == reflect.Slice {
			return fmt.Sprintf("At least %s items required", e.Param())
		}
		return fmt.Sprintf("Minimum value is %s", e.Param())
	case "max":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Maximum %s characters allowed", e.Param())
		} else if e.Kind() == reflect.Slice {
			return fmt.Sprintf("Maximum %s items allowed", e.Param())
		}
		return fmt.Sprintf("Maximum value is %s", e.Param())
	case "in":
		return fmt.Sprintf("Must be one of: %s", strings.Replace(e.Param(), ",", ", ", -1))
	case "unique":
		return "Contains duplicate values"
	case "dive":
		return fmt.Sprintf("Invalid element: %s", e.Error())
	case "email":
		return "Invalid email format"
	default:
		return e.Error()
	}
}

// Example Usage
func main() {
	r := mux.NewRouter()
	
	// User creation endpoint (POST /users)
	r.HandleFunc("/users", ValidateRequest([]ValidationConfig{
		{
			FieldName:    "Name",
			Parser:       BodyParser,
			Required:     true,
			ValidateTags: "required,min=3,max=50",
		},
		{
			FieldName:    "Email",
			Parser:       BodyParser,
			Required:     true,
			ValidateTags: "required,email",
		},
		{
			FieldName:    "Tags",
			Parser:       BodyParser,
			Required:     false,
			ValidateTags: "omitempty,unique,dive,min=2,max=20",
		},
	}, "create")).Methods("POST")
	
	// User update endpoint (PUT /users/{user_id})
	r.HandleFunc("/users/{user_id}", ValidateRequest([]ValidationConfig{
		{
			FieldName:    "UserID",
			Parser:       ParamParser,
			Required:     true,
			ValidateTags: "required,min=1",
		},
		{
			FieldName:    "Name",
			Parser:       BodyParser,
			Required:     false,
			ValidateTags: "omitempty,min=3,max=50",
		},
		{
			FieldName:    "IDs",
			Parser:       QueryParser,
			Required:     false,
			ValidateTags: "omitempty,unique,dive,min=1",
		},
	}, "update")).Methods("PUT")
	
	// Search endpoint (GET /search)
	r.HandleFunc("/search", ValidateRequest([]ValidationConfig{
		{
			FieldName:    "Tags",
			Parser:       QueryParser,
			Required:     true,
			ValidateTags: "required,min=1,max=5,dive,in=tech,sports,politics",
		},
		{
			FieldName:    "Rating",
			Parser:       QueryParser,
			Required:     false,
			Default:      "5",
			ValidateTags: "omitempty,min=1,max=5",
		},
	}, "search")).Methods("GET")
	
	http.ListenAndServe(":8080", r)
}
