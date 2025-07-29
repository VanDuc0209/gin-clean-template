package validation

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/duccv/go-clean-template/internal/constant"
)

type Validator struct {
}

func NewValidator() Validator {
	return Validator{}
}

var validate *validator.Validate = validator.New()

func isEmptyInterface[T any]() bool {
	t := reflect.TypeOf((*T)(nil)).Elem()
	return t == reflect.TypeOf((*any)(nil)).Elem()
}

func Validate[B any, P any, Q any]() gin.HandlerFunc {
	return func(c *gin.Context) {
		var error string
		c.Set("error", &error)
		resData := constant.INVALID_REQUEST

		// --- Body ---
		if !isEmptyInterface[B]() {
			var body B

			rawData, err := io.ReadAll(c.Request.Body)
			if err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))

			if err := c.ShouldBindJSON(&body); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}
			if err := validate.Struct(body); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}

			// Ghi lại để các middleware khác vẫn dùng được
			c.Request.Body = io.NopCloser(bytes.NewBuffer(rawData))
			c.Set("validatedBody", body)
		}

		// --- Params ---
		if !isEmptyInterface[P]() {
			var params P

			// Clone lại Params
			originalParams := c.Params

			if err := c.ShouldBindUri(&params); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}
			if err := validate.Struct(params); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}

			// Gán lại param
			c.Params = originalParams
			c.Set("validatedParams", params)
		}

		// --- Query ---
		if !isEmptyInterface[Q]() {
			var query Q

			// Clone query string
			originalQuery := c.Request.URL.RawQuery
			originalValues, _ := url.ParseQuery(originalQuery)

			if err := c.ShouldBindQuery(&query); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}
			if err := validate.Struct(query); err != nil {
				error = err.Error()
				resData.Error = err.Error()
				c.AbortWithStatusJSON(http.StatusBadRequest, resData)
				return
			}

			// Reset lại query
			c.Request.URL.RawQuery = originalValues.Encode()
			c.Set("validatedQuery", query)
		}

		c.Next()
	}
}
