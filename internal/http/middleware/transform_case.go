package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// TransformCaseMiddleware преобразует входящие JSON запросы из camelCase в snake_case
// и исходящие JSON ответы из snake_case в camelCase
func TransformCaseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && isJSONRequest(r) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				var data interface{}
				if err := json.Unmarshal(bodyBytes, &data); err == nil {
					transformedData := convertKeysToSnakeCase(data)
					transformedBytes, err := json.Marshal(transformedData)
					if err == nil {
						r.Body = io.NopCloser(bytes.NewReader(transformedBytes))
						r.ContentLength = int64(len(transformedBytes))
					}
				}
			}
		}

		responseWriter := &responseWriterWrapper{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(responseWriter, r)

		if isJSONResponse(responseWriter) && responseWriter.body.Len() > 0 {
			var data interface{}
			if err := json.Unmarshal(responseWriter.body.Bytes(), &data); err == nil {
				transformedData := convertKeysToCamelCase(data)
				transformedBytes, err := json.Marshal(transformedData)
				if err == nil {
					responseWriter.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
					responseWriter.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(transformedBytes)))
					if responseWriter.statusCode != 0 {
						responseWriter.ResponseWriter.WriteHeader(responseWriter.statusCode)
					}
					responseWriter.ResponseWriter.Write(transformedBytes)
					return
				}
			}
		}

		if responseWriter.body.Len() > 0 {
			if responseWriter.statusCode != 0 {
				responseWriter.ResponseWriter.WriteHeader(responseWriter.statusCode)
			}
			responseWriter.ResponseWriter.Write(responseWriter.body.Bytes())
		}
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// isJSONRequest проверяет, является ли запрос JSON
func isJSONRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// isJSONResponse проверяет, является ли ответ JSON
func isJSONResponse(w *responseWriterWrapper) bool {
	contentType := w.Header().Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// convertKeysToSnakeCase рекурсивно преобразует ключи map/array в snake_case
func convertKeysToSnakeCase(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			newKey := toSnakeCase(key)
			result[newKey] = convertKeysToSnakeCase(value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertKeysToSnakeCase(item)
		}
		return result
	default:
		return v
	}
}

// convertKeysToCamelCase рекурсивно преобразует ключи map/array в camelCase
func convertKeysToCamelCase(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			newKey := toCamelCase(key)
			result[newKey] = convertKeysToCamelCase(value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertKeysToCamelCase(item)
		}
		return result
	default:
		return v
	}
}

// toSnakeCase преобразует строку в snake_case
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	if s == strings.ToLower(s) && !strings.Contains(s, "_") {
		return s
	}

	var result strings.Builder
	var prev rune

	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 && prev != '_' && prev != '-' {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else if r == '-' || r == ' ' {
			if prev != '_' && prev != '-' && prev != ' ' {
				result.WriteRune('_')
			}
		} else {
			result.WriteRune(r)
		}
		prev = r
	}

	return result.String()
}

// isCamelCase проверяет, является ли строка уже в формате camelCase
func isCamelCase(s string) bool {
	if s == "" {
		return false
	}

	if strings.Contains(s, "_") || strings.Contains(s, "-") {
		return false
	}

	if s == strings.ToLower(s) {
		return false
	}

	if len(s) > 0 {
		first := rune(s[0])
		if !unicode.IsLower(first) {
			return false
		}

		hasUpper := false
		for _, r := range s[1:] {
			if unicode.IsUpper(r) {
				hasUpper = true
				break
			}
		}

		return hasUpper
	}

	return false
}

// toCamelCase преобразует строку в camelCase
func toCamelCase(s string) string {
	if s == "" {
		return s
	}

	if isCamelCase(s) {
		return s
	}

	if strings.Contains(s, "_") || strings.Contains(s, "-") {
		s = regexp.MustCompile(`[-_\s]+`).ReplaceAllString(s, " ")

		words := strings.Fields(s)
		if len(words) == 0 {
			return s
		}

		var result strings.Builder

		result.WriteString(strings.ToLower(words[0]))

		for i := 1; i < len(words); i++ {
			word := words[i]
			if len(word) > 0 {
				result.WriteString(strings.ToUpper(string(word[0])))
				if len(word) > 1 {
					result.WriteString(strings.ToLower(word[1:]))
				}
			}
		}

		return result.String()
	}

	if s == strings.ToLower(s) {
		return s
	}

	var result strings.Builder
	var wordStart int
	var prevUpper bool
	firstWord := true

	for i, r := range s {
		isUpper := unicode.IsUpper(r)

		if i > 0 {
			if isUpper && !prevUpper {
				if wordStart < i {
					word := s[wordStart:i]
					if firstWord {
						result.WriteString(strings.ToLower(word))
						firstWord = false
					} else {
						if len(word) > 0 {
							result.WriteString(strings.ToUpper(string(word[0])))
							if len(word) > 1 {
								result.WriteString(strings.ToLower(word[1:]))
							}
						}
					}
				}
				wordStart = i
			}
		}

		prevUpper = isUpper
	}

	if wordStart < len(s) {
		word := s[wordStart:]
		if firstWord {
			result.WriteString(strings.ToLower(word))
		} else {
			if len(word) > 0 {
				result.WriteString(strings.ToUpper(string(word[0])))
				if len(word) > 1 {
					result.WriteString(strings.ToLower(word[1:]))
				}
			}
		}
	}

	if result.Len() == 0 {
		return strings.ToLower(s)
	}

	return result.String()
}
