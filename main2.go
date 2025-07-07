package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// ======================
// Domain Layer
// ======================

type User struct {
	ID       uint64   `json:"id" validate:"required,update"`
	Name     string   `json:"name" validate:"required,min=3,max=50,create"`
	Email    string   `json:"email" validate:"required,email"`
	Tags     []string `json:"tags" validate:"dive,min=2,max=20"`
	Role     string   `json:"role" validate:"in=user,admin,moderator"`
}

// ======================
// Validation Layer
// ======================

var validate = validator.New()

func init() {
	validate.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if field.CanUint() {
			return field.Uint()
		}
		return nil
	}, uint64(0))
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func validateStruct(data interface{}) []ValidationError {
	var errors []ValidationError
	
	if err := validate.Struct(data); err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			errors = append(errors, ValidationError{
				Field:   err.StructField(),
				Message: getErrorMessage(err),
			})
		}
	}
	return errors
}

func getErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "min":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Must be at least %s characters", e.Param())
		}
		return fmt.Sprintf("Must be at least %s", e.Param())
	case "max":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Must be at most %s characters", e.Param())
		}
		return fmt.Sprintf("Must be at most %s", e.Param())
	case "email":
		return "Invalid email format"
	case "in":
		return fmt.Sprintf("Must be one of: %s", strings.Replace(e.Param(), ",", ", ", -1))
	default:
		return e.Error()
	}
}

// ======================
// Transport Layer
// ======================

type RequestParser struct {
	Ctx *fiber.Ctx
}

func (p *RequestParser) ParseBody(dest interface{}) error {
	if err := p.Ctx.BodyParser(dest); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	return nil
}

func (p *RequestParser) ParseQuery(dest interface{}) error {
	query := p.Ctx.Queries()
	if err := mapToStruct(query, dest); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid query parameters")
	}
	return nil
}

func mapToStruct(m map[string]string, dest interface{}) error {
	val := reflect.ValueOf(dest).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		structField := typ.Field(i)
		tag := structField.Tag.Get("query")

		if tag == "" {
			continue
		}

		if value, exists := m[tag]; exists {
			switch field.Kind() {
			case reflect.String:
				field.SetString(value)
			case reflect.Int, reflect.Int64:
				num, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return err
				}
				field.SetInt(num)
			case reflect.Uint, reflect.Uint64:
				num, err := strconv.ParseUint(value, 10, 64)
				if err != nil {
					return err
				}
				field.SetUint(num)
			case reflect.Slice:
				values := strings.Split(value, ",")
				if field.Type().Elem().Kind() == reflect.String {
					field.Set(reflect.ValueOf(values))
				}
			}
		}
	}
	return nil
}

// ======================
// Handler Layer
// ======================

type UserHandler struct {
	Service UserService
}

type UserService interface {
	CreateUser(user *User) error
	GetUser(id uint64) (*User, error)
}

func (h *UserHandler) CreateUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Parse Request
		var user User
		parser := RequestParser{Ctx: c}
		if err := parser.ParseBody(&user); err != nil {
			return err
		}

		// 2. Validate
		if errors := validateStruct(&user); len(errors) > 0 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"errors": errors,
			})
		}

		// 3. Business Logic
		if err := h.Service.CreateUser(&user); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to create user")
		}

		// 4. Response
		return c.Status(fiber.StatusCreated).JSON(user)
	}
}

func (h *UserHandler) GetUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Parse Request
		id, err := strconv.ParseUint(c.Params("id"), 10, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
		}

		// 2. Business Logic
		user, err := h.Service.GetUser(id)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}

		// 3. Response
		return c.JSON(user)
	}
}

// ======================
// Application Layer
// ======================

type App struct {
	UserHandler *UserHandler
}

func NewApp(service UserService) *App {
	return &App{
		UserHandler: &UserHandler{Service: service},
	}
}

// ======================
// Main
// ======================

func main() {
	app := fiber.New()

	// Initialize layers
	userService := &MockUserService{} // Implement your actual service
	myApp := NewApp(userService)

	// Routes
	api := app.Group("/api")
	api.Post("/users", myApp.UserHandler.CreateUser())
	api.Get("/users/:id", myApp.UserHandler.GetUser())

	app.Listen(":3000")
}

// Mock Service Implementation
type MockUserService struct{}

func (m *MockUserService) CreateUser(user *User) error {
	// Implement your business logic
	return nil
}

func (m *MockUserService) GetUser(id uint64) (*User, error) {
	// Implement your business logic
	return &User{ID: id, Name: "Test User"}, nil
}
