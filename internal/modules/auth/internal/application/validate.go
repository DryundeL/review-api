package application

import (
	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/domain"
)

type ValidateInitDataHandler struct {
	validator *domain.Validator
}

func NewValidateInitDataHandler(v *domain.Validator) *ValidateInitDataHandler {
	return &ValidateInitDataHandler{validator: v}
}

func (h *ValidateInitDataHandler) Handle(initData string) (contract.TelegramIdentity, error) {
	return h.validator.Validate(initData)
}
