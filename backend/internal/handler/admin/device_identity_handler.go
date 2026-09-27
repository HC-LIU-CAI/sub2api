package admin

import (
	"io"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// deviceIdentityEnsureRequest controls whether existing identities are replaced.
type deviceIdentityEnsureRequest struct {
	Force bool `json:"force"`
}

type deviceIdentityEnsureResponse struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// EnsureDeviceIdentities fills missing provider device identities, or rotates
// all eligible identities when force=true. Identities live in account extra so
// this operation does not require a schema migration.
func (h *AccountHandler) EnsureDeviceIdentities(c *gin.Context) {
	var req deviceIdentityEnsureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// An empty body is equivalent to the default {force:false}; malformed JSON
		// remains a client error so callers do not accidentally rotate identities.
		if err != io.EOF {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	accounts, err := h.listDeviceIdentityAccounts(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := deviceIdentityEnsureResponse{}
	for i := range accounts {
		account := &accounts[i]
		key, ok := deviceIdentityExtraKey(account)
		if !ok {
			continue
		}
		if !req.Force && accountDeviceIdentity(account, key) != "" {
			result.Skipped++
			continue
		}
		if err := h.adminService.UpdateAccountExtra(c.Request.Context(), account.ID, map[string]any{
			key: uuid.NewString(),
		}); err != nil {
			result.Failed++
			continue
		}
		result.Updated++
	}

	response.Success(c, result)
}

// ResetDeviceIdentity creates a new identity for one eligible OAuth account.
func (h *AccountHandler) ResetDeviceIdentity(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, errors.BadRequest("INVALID_ACCOUNT_ID", "Invalid account ID"))
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	key, ok := deviceIdentityExtraKey(account)
	if !ok {
		response.ErrorFrom(c, errors.BadRequest(
			"DEVICE_IDENTITY_UNSUPPORTED_ACCOUNT",
			"device identity is only supported for OpenAI or Anthropic OAuth accounts",
		))
		return
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), account.ID, map[string]any{
		key: uuid.NewString(),
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"updated": true, "account_id": account.ID})
}

func (h *AccountHandler) listDeviceIdentityAccounts(c *gin.Context) ([]service.Account, error) {
	ctx := c.Request.Context()
	accounts := make([]service.Account, 0)
	for _, candidate := range []struct {
		platform string
		typeName string
	}{
		{service.PlatformOpenAI, service.AccountTypeOAuth},
		{service.PlatformAnthropic, service.AccountTypeOAuth},
		{service.PlatformAnthropic, service.AccountTypeSetupToken},
	} {
		items, err := h.adminService.ListAccountsForSchedulerScoreFilter(ctx, candidate.platform, candidate.typeName, "", "", 0, "")
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, items...)
	}
	return accounts, nil
}

func deviceIdentityExtraKey(account *service.Account) (string, bool) {
	if account == nil {
		return "", false
	}
	switch {
	case account.Platform == service.PlatformOpenAI && account.Type == service.AccountTypeOAuth:
		return "openai_device_id", true
	case account.Platform == service.PlatformAnthropic &&
		(account.Type == service.AccountTypeOAuth || account.Type == service.AccountTypeSetupToken):
		return "claude_user_id", true
	default:
		return "", false
	}
}

func accountDeviceIdentity(account *service.Account, key string) string {
	if account == nil || account.Extra == nil {
		return ""
	}
	value, _ := account.Extra[key].(string)
	return strings.TrimSpace(value)
}
