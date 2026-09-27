package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type deviceIdentityUpdate struct {
	accountID int64
	updates   map[string]any
}

type deviceIdentityAdminService struct {
	*stubAdminService
	accountsByFilter map[string][]service.Account
	accountsByID     map[int64]*service.Account
	listErr          error
	getErr           error
	updateErrByID    map[int64]error
	updates          []deviceIdentityUpdate
}

func newDeviceIdentityAdminService() *deviceIdentityAdminService {
	return &deviceIdentityAdminService{
		stubAdminService: newStubAdminService(),
		accountsByFilter: map[string][]service.Account{},
		accountsByID:     map[int64]*service.Account{},
		updateErrByID:    map[int64]error{},
	}
}

func (s *deviceIdentityAdminService) ListAccountsForSchedulerScoreFilter(
	_ context.Context,
	platform, accountType, _, _ string,
	_ int64,
	_ string,
) ([]service.Account, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.accountsByFilter[platform+"/"+accountType], nil
}

func (s *deviceIdentityAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if account, ok := s.accountsByID[id]; ok {
		return account, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func (s *deviceIdentityAdminService) UpdateAccountExtra(_ context.Context, id int64, updates map[string]any) error {
	copied := make(map[string]any, len(updates))
	for key, value := range updates {
		copied[key] = value
	}
	s.updates = append(s.updates, deviceIdentityUpdate{accountID: id, updates: copied})
	return s.updateErrByID[id]
}

func setupDeviceIdentityRouter(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/device-identity/ensure", handler.EnsureDeviceIdentities)
	router.POST("/api/v1/admin/accounts/:id/device-identity/reset", handler.ResetDeviceIdentity)
	return router
}

func deviceIdentityAccount(id int64, platform, accountType string, extra map[string]any) service.Account {
	return service.Account{
		ID:       id,
		Platform: platform,
		Type:     accountType,
		Extra:    extra,
		Status:   service.StatusActive,
	}
}

func decodeResponseData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload.Data
}

func TestEnsureDeviceIdentitiesSkipsExistingAndUsesProviderKeys(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	adminSvc.accountsByFilter["openai/oauth"] = []service.Account{
		deviceIdentityAccount(101, service.PlatformOpenAI, service.AccountTypeOAuth, nil),
	}
	adminSvc.accountsByFilter["anthropic/oauth"] = []service.Account{
		deviceIdentityAccount(102, service.PlatformAnthropic, service.AccountTypeOAuth, map[string]any{
			"claude_user_id": "existing-claude-id",
		}),
	}
	adminSvc.accountsByFilter["anthropic/setup-token"] = []service.Account{
		deviceIdentityAccount(103, service.PlatformAnthropic, service.AccountTypeSetupToken, nil),
	}
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/device-identity/ensure", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeResponseData(t, rec.Body.Bytes())
	require.Equal(t, float64(2), data["updated"])
	require.Equal(t, float64(1), data["skipped"])
	require.Equal(t, float64(0), data["failed"])
	require.Len(t, adminSvc.updates, 2)

	updates := map[int64]map[string]any{}
	for _, update := range adminSvc.updates {
		updates[update.accountID] = update.updates
	}
	require.Contains(t, updates[101], "openai_device_id")
	require.Contains(t, updates[103], "claude_user_id")
	require.NotContains(t, updates, int64(102))
	for _, update := range adminSvc.updates {
		for _, value := range update.updates {
			id, ok := value.(string)
			require.True(t, ok)
			require.NoError(t, uuid.Validate(id))
		}
	}
}

func TestEnsureDeviceIdentitiesForceRotatesExistingAndCountsFailures(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	adminSvc.accountsByFilter["openai/oauth"] = []service.Account{
		deviceIdentityAccount(201, service.PlatformOpenAI, service.AccountTypeOAuth, map[string]any{
			"openai_device_id": "old-openai-id",
		}),
	}
	adminSvc.accountsByFilter["anthropic/oauth"] = []service.Account{
		deviceIdentityAccount(202, service.PlatformAnthropic, service.AccountTypeOAuth, map[string]any{
			"claude_user_id": "old-claude-id",
		}),
	}
	adminSvc.accountsByFilter["anthropic/setup-token"] = []service.Account{
		deviceIdentityAccount(203, service.PlatformAnthropic, service.AccountTypeSetupToken, nil),
	}
	adminSvc.updateErrByID[202] = errors.New("write failed")
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/accounts/device-identity/ensure",
		bytes.NewBufferString(`{"force":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeResponseData(t, rec.Body.Bytes())
	require.Equal(t, float64(2), data["updated"])
	require.Equal(t, float64(0), data["skipped"])
	require.Equal(t, float64(1), data["failed"])
	require.Len(t, adminSvc.updates, 3)
}

func TestEnsureDeviceIdentitiesRejectsMalformedJSONAndPropagatesListErrors(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/accounts/device-identity/ensure",
		bytes.NewBufferString(`{"force":`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	adminSvc.listErr = errors.New("list failed")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/device-identity/ensure", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestResetDeviceIdentityGeneratesIdentityForEligibleAccount(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	adminSvc.accountsByID[301] = ptrAccount(deviceIdentityAccount(
		301, service.PlatformOpenAI, service.AccountTypeOAuth, map[string]any{"keep": "value"},
	))
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/301/device-identity/reset", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeResponseData(t, rec.Body.Bytes())
	require.Equal(t, true, data["updated"])
	require.Equal(t, float64(301), data["account_id"])
	require.Len(t, adminSvc.updates, 1)
	require.Equal(t, int64(301), adminSvc.updates[0].accountID)
	deviceID, ok := adminSvc.updates[0].updates["openai_device_id"].(string)
	require.True(t, ok)
	require.NoError(t, uuid.Validate(deviceID))
}

func TestResetDeviceIdentityRejectsInvalidIDAndUnsupportedAccount(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	adminSvc.accountsByID[302] = ptrAccount(deviceIdentityAccount(
		302, service.PlatformAnthropic, service.AccountTypeAPIKey, nil,
	))
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/not-an-id/device-identity/reset", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var invalidIDPayload struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &invalidIDPayload))
	require.Equal(t, "INVALID_ACCOUNT_ID", invalidIDPayload.Reason)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/302/device-identity/reset", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var unsupportedPayload struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &unsupportedPayload))
	require.Equal(t, "DEVICE_IDENTITY_UNSUPPORTED_ACCOUNT", unsupportedPayload.Reason)
	require.Empty(t, adminSvc.updates)
}

func TestResetDeviceIdentityPropagatesReadAndWriteErrors(t *testing.T) {
	adminSvc := newDeviceIdentityAdminService()
	adminSvc.getErr = errors.New("read failed")
	router := setupDeviceIdentityRouter(adminSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/303/device-identity/reset", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	adminSvc.getErr = nil
	adminSvc.accountsByID[303] = ptrAccount(deviceIdentityAccount(
		303, service.PlatformAnthropic, service.AccountTypeOAuth, nil,
	))
	adminSvc.updateErrByID[303] = errors.New("write failed")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/303/device-identity/reset", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Len(t, adminSvc.updates, 1)
}

func ptrAccount(account service.Account) *service.Account {
	return &account
}
