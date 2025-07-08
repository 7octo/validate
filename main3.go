package main

import (
	"encoding"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Custom types with UnmarshalText implementations
type Uint64Slice []uint64
type StringSlice []string
type FlexibleBool bool

// AppStruct with custom types
type AppStruct struct {
	F Uint64Slice  `json:"f" form:"f" query:"f"`
	S StringSlice  `json:"s" form:"s" query:"s"`
	B string       `json:"b" form:"b" query:"b"`
	G int          `json:"g" form:"g" query:"g"`
	V FlexibleBool `json:"v" form:"v" query:"v"`
}

// Implement UnmarshalText for Uint64Slice
func (u *Uint64Slice) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	*u = make(Uint64Slice, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		num, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint64 value: %q", part)
		}
		*u = append(*u, num)
	}
	return nil
}

// Implement encoding.TextUnmarshaler for Fiber form parsing
func (u *Uint64Slice) UnmarshalForm(text []byte) error {
	return u.UnmarshalText(text)
}

// Implement UnmarshalText for StringSlice
func (s *StringSlice) UnmarshalText(text []byte) error {
	*s = strings.Split(string(text), ",")
	for i, val := range *s {
		(*s)[i] = strings.TrimSpace(val)
	}
	return nil
}

func (s *StringSlice) UnmarshalForm(text []byte) error {
	return s.UnmarshalText(text)
}

// Implement UnmarshalText for FlexibleBool
func (b *FlexibleBool) UnmarshalText(text []byte) error {
	s := strings.ToLower(string(text))
	switch s {
	case "true", "1", "on", "yes":
		*b = true
	case "false", "0", "off", "no", "":
		*b = false
	default:
		return fmt.Errorf("invalid boolean value: %q", s)
	}
	return nil
}

func (b *FlexibleBool) UnmarshalForm(text []byte) error {
	return b.UnmarshalText(text)
}

// Ensure our types implement all necessary interfaces
var (
	_ encoding.TextUnmarshaler = (*Uint64Slice)(nil)
	_ encoding.TextUnmarshaler = (*StringSlice)(nil)
	_ encoding.TextUnmarshaler = (*FlexibleBool)(nil)
)

func main() {
	app := fiber.New()

	app.Post("/test", func(c *fiber.Ctx) error {
		var input AppStruct

		// Parse input from all sources (JSON, Form, Query)
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Bad Request",
				"details": err.Error(),
			})
		}

		// For GET requests or form data in query params
		if c.Method() == fiber.MethodGet || len(c.Body()) == 0 {
			if err := parseQueryParams(c, &input); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error":   "Bad Request",
					"details": err.Error(),
				})
			}
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data":    input,
		})
	})

	app.Listen(":3000")
}

func parseQueryParams(c *fiber.Ctx, input *AppStruct) error {
	// Fiber's query parser will use our UnmarshalText implementations
	if err := c.QueryParser(input); err != nil {
		return err
	}
	return nil
}
