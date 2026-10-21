package domain

import (
	"regexp"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
)

var metricRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,63}$`)

type Achievement struct {
	ID           uuid.UUID
	Code         string
	Title        string
	Description  string
	Metric       string
	Target       int
	RewardPoints int
	IsActive     bool
	SortOrder    int
}

func NewAchievement(code, title, description, metric string, target, reward, sortOrder int, active bool) (Achievement, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	metric = strings.TrimSpace(strings.ToLower(metric))
	if !ValidCode(code) {
		return Achievement{}, contract.FieldError{Field: "code"}
	}
	if len(title) < 1 || len(title) > 64 {
		return Achievement{}, contract.FieldError{Field: "title"}
	}
	if len(description) < 1 || len(description) > 256 {
		return Achievement{}, contract.FieldError{Field: "description"}
	}
	if !ValidMetric(metric) {
		return Achievement{}, contract.FieldError{Field: "metric"}
	}
	if target < 1 {
		return Achievement{}, contract.FieldError{Field: "target"}
	}
	if reward < 0 || reward > 10000 {
		return Achievement{}, contract.FieldError{Field: "rewardPoints"}
	}
	if sortOrder < 0 || sortOrder > 10000 {
		return Achievement{}, contract.FieldError{Field: "sortOrder"}
	}
	return Achievement{
		ID: uuid.New(), Code: code, Title: title, Description: description,
		Metric: metric, Target: target, RewardPoints: reward, IsActive: active, SortOrder: sortOrder,
	}, nil
}

func ValidMetric(metric string) bool {
	return metricRe.MatchString(metric)
}

// TypeMetric returns reviews_type_<mediaType> or empty if mediaType is invalid.
func TypeMetric(mediaType string) string {
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))
	if !ValidMetric(mediaType) {
		return ""
	}
	return "reviews_type_" + mediaType
}
