package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
)

// RememberFactAction saves an explicit user fact to long-term memory.
type RememberFactAction struct {
	repo       ports.ImprintRepository
	spiritName string
}

func NewRememberFactAction(repo ports.ImprintRepository, spiritName string) *RememberFactAction {
	return &RememberFactAction{repo: repo, spiritName: spiritName}
}

func (a *RememberFactAction) GetName() string { return "remember_fact" }

func (a *RememberFactAction) GetDescription() string {
	return "Saves a fact about the user to long-term memory so it can be recalled in future conversations."
}

func (a *RememberFactAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "The fact to remember about the user.",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "Category: preference, personal, business, or context.",
				"enum":        []string{"preference", "personal", "business", "context"},
			},
		},
		"required": []string{"fact", "category"},
	}
}

func (a *RememberFactAction) GetCategory() ports.ActionCategory { return ports.ActionModification }
func (a *RememberFactAction) IsCritical() bool                  { return false }

func (a *RememberFactAction) Validate(args map[string]any) error {
	if fact, _ := args["fact"].(string); fact == "" {
		return domainerrors.NewBusinessError("fact is required")
	}
	return nil
}

func (a *RememberFactAction) Execute(ctx context.Context, args map[string]any) (string, error) {
	fact, _ := args["fact"].(string)
	categoryStr, _ := args["category"].(string)
	category := entities.ImprintCategory(categoryStr)
	if category == "" {
		category = entities.ImprintCategoryContext
	}

	pulse, _ := ctx.Value(ports.PulseContextKey{}).(*entities.Pulse)
	if pulse == nil || pulse.ContactPhone == "" {
		return "", domainerrors.NewBusinessError("no user context available to associate memory")
	}

	now := time.Now().UTC()
	imprint := entities.Imprint{
		ID:         helpers.GenerateRandomID(),
		UserKey:    pulse.ContactPhone,
		SpiritName: a.spiritName,
		Fact:       fact,
		Category:   category,
		Source:     entities.ImprintSourceExplicit,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := a.repo.Save(ctx, imprint); err != nil {
		return "", domainerrors.NewInfrastructureError("failed to save memory", err)
	}

	return fmt.Sprintf("Remembered: %s", fact), nil
}

// ForgetFactAction deletes user facts matching a substring from long-term memory.
type ForgetFactAction struct {
	repo       ports.ImprintRepository
	spiritName string
}

func NewForgetFactAction(repo ports.ImprintRepository, spiritName string) *ForgetFactAction {
	return &ForgetFactAction{repo: repo, spiritName: spiritName}
}

func (a *ForgetFactAction) GetName() string { return "forget_fact" }

func (a *ForgetFactAction) GetDescription() string {
	return "Removes facts about the user from long-term memory that match the given substring."
}

func (a *ForgetFactAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact_substring": map[string]any{
				"type":        "string",
				"description": "Substring to match against stored facts. All matching facts will be deleted.",
			},
		},
		"required": []string{"fact_substring"},
	}
}

func (a *ForgetFactAction) GetCategory() ports.ActionCategory { return ports.ActionModification }
func (a *ForgetFactAction) IsCritical() bool                  { return false }

func (a *ForgetFactAction) Validate(args map[string]any) error {
	if sub, _ := args["fact_substring"].(string); sub == "" {
		return domainerrors.NewBusinessError("fact_substring is required")
	}
	return nil
}

func (a *ForgetFactAction) Execute(ctx context.Context, args map[string]any) (string, error) {
	sub, _ := args["fact_substring"].(string)

	pulse, _ := ctx.Value(ports.PulseContextKey{}).(*entities.Pulse)
	if pulse == nil || pulse.ContactPhone == "" {
		return "", domainerrors.NewBusinessError("no user context available")
	}

	imprints, err := a.repo.GetByUserKey(ctx, pulse.ContactPhone, a.spiritName)
	if err != nil {
		return "", domainerrors.NewInfrastructureError("failed to load memories", err)
	}

	deleted := 0
	for _, imp := range imprints {
		if strings.Contains(strings.ToLower(imp.Fact), strings.ToLower(sub)) {
			if err := a.repo.Delete(ctx, imp.ID); err == nil {
				deleted++
			}
		}
	}

	return fmt.Sprintf("Forgot %d fact(s) matching %q", deleted, sub), nil
}
