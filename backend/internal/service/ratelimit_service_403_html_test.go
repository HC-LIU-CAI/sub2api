//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// countingOpenAI403CounterCache 在既有桩的基础上记录递增次数。
// HTML 403 不仅不能处罚账号，连计数都不能加——否则后续真实的账号级 403
// 会踩着这些"白涨"的计数提前触发永久禁用。
type countingOpenAI403CounterCache struct {
	openAI403CounterCacheStub
	increments int
}

func (s *countingOpenAI403CounterCache) IncrementOpenAI403Count(ctx context.Context, accountID int64, window int) (int64, error) {
	s.increments++
	return s.openAI403CounterCacheStub.IncrementOpenAI403Count(ctx, accountID, window)
}

type openAI403TestHarness struct {
	svc     *RateLimitService
	repo    *rateLimitAccountRepoStub
	counter *countingOpenAI403CounterCache
	blocker *runtimeBlockRecorder
	account *Account
}

func newOpenAI403TestHarness(t *testing.T, accountID int64, counts ...int64) *openAI403TestHarness {
	t.Helper()
	repo := &rateLimitAccountRepoStub{}
	counter := &countingOpenAI403CounterCache{openAI403CounterCacheStub: openAI403CounterCacheStub{counts: counts}}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetOpenAI403CounterCache(counter)
	svc.SetAccountRuntimeBlocker(blocker)
	return &openAI403TestHarness{
		svc:     svc,
		repo:    repo,
		counter: counter,
		blocker: blocker,
		account: &Account{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	}
}

func (h *openAI403TestHarness) handle(body string) bool {
	return h.svc.HandleUpstreamError(
		context.Background(), h.account, http.StatusForbidden, http.Header{}, []byte(body),
	)
}

func (h *openAI403TestHarness) requireNoAccountPenalty(t *testing.T) {
	t.Helper()
	require.Equal(t, 0, h.repo.setErrorCalls, "端点级 403 不得永久禁用账号")
	require.Equal(t, 0, h.repo.tempCalls, "端点级 403 不得把账号设为临时不可调度")
	require.Empty(t, h.blocker.accounts, "端点级 403 不得触发调度阻断通知")
	require.Equal(t, 0, h.counter.increments, "端点级 403 不得递增连续 403 计数")
}

// issue #5334：无效的 /v1/responses 子路径被转发后，上游代理在到达 OpenAI API
// 之前回 HTML 403 页面。这是链路/端点级响应，不是账号凭据失效的证据。
const openAI403HTMLBody = "<!DOCTYPE html>\n<html><head><title>403 Forbidden</title></head>" +
	"<body><h1>403 Forbidden</h1></body></html>"

const openAI403Cloudflare1010Body = "error code: 1010\n"

func TestHandleUpstreamError_OpenAIHTML403DoesNotPenalizeAccount(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"doctype_prefixed", openAI403HTMLBody},
		{"bare_html_tag", "<html><body>403 Forbidden</body></html>"},
		{"leading_whitespace_and_uppercase", "\n\t  <!DOCTYPE HTML><html><body>Forbidden</body></html>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOpenAI403TestHarness(t, 501, 1)

			shouldDisable := h.handle(tc.body)

			require.False(t, shouldDisable, "HTML 403 不得判定账号应下线")
			h.requireNoAccountPenalty(t)
		})
	}
}

// 一个持有 API Key 的调用方反复打无效子路径时，既有实现会在第
// openAI403DisableThreshold 次把账号永久禁用。修复后连续多少次都不该升级。
func TestHandleUpstreamError_OpenAIHTML403RepeatedNeverEscalates(t *testing.T) {
	h := newOpenAI403TestHarness(t, 502, 1, 2, 3, 4, 5)

	for i := 0; i < openAI403DisableThreshold+2; i++ {
		require.False(t, h.handle(openAI403HTMLBody), "第 %d 次 HTML 403 仍不得判定账号应下线", i+1)
	}

	h.requireNoAccountPenalty(t)
}

func TestHandleUpstreamError_OpenAICloudflare1010DoesNotPenalizeAccount(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformOpenCodeGo} {
		t.Run(platform, func(t *testing.T) {
			repo := &rateLimitAccountRepoStub{}
			counter := &countingOpenAI403CounterCache{
				openAI403CounterCacheStub: openAI403CounterCacheStub{counts: []int64{openAI403DisableThreshold}},
			}
			blocker := &runtimeBlockRecorder{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			svc.SetOpenAI403CounterCache(counter)
			svc.SetAccountRuntimeBlocker(blocker)
			account := &Account{ID: 601, Platform: platform, Type: AccountTypeAPIKey}

			shouldDisable := svc.HandleUpstreamError(
				context.Background(), account, http.StatusForbidden, http.Header{}, []byte(openAI403Cloudflare1010Body),
			)

			require.False(t, shouldDisable, "Cloudflare 1010 不得判定账号应下线")
			require.Equal(t, 0, counter.increments, "Cloudflare 1010 不得递增账号 403 计数")
			require.Equal(t, 0, repo.setErrorCalls)
			require.Equal(t, 0, repo.tempCalls)
			require.Empty(t, blocker.accounts)
		})
	}
}

// 普通 OpenAI 403 只触发 failover，不应累计账号计数、临时冷却或永久下线。
// 这覆盖结构化 JSON 和纯文本响应，避免只豁免 HTML/Cloudflare 页面而遗漏
// 生产环境中同样属于请求级拒绝的普通 403。
func TestHandleUpstreamError_OpenAIStructured403FailoverOnly(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"structured_json", `{"error":{"message":"Your account is not authorized"}}`},
		{"plain_text", "Forbidden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOpenAI403TestHarness(t, 503, 1, 2, 3, 4)

			for i := 0; i < openAI403DisableThreshold+1; i++ {
				require.False(t, h.handle(tc.body), "第 %d 次普通 OpenAI 403 仍不得下线账号", i+1)
			}
			h.requireNoAccountPenalty(t)
		})
	}
}

func TestIsOpenAI403AccountSignal(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		body string
		want bool
	}{
		{"invalid_api_key", "", `{"error":{"code":"invalid_api_key"}}`, true},
		{"workspace_suspended", "", `{"error":{"message":"workspace has been suspended"}}`, true},
		{"quota_exhausted", "", `{"error":{"type":"access_terminated_error"}}`, true},
		{"ordinary_request_rejection", "request was rejected", `{"error":{"message":"forbidden"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAI403AccountSignal(tc.msg, []byte(tc.body)))
		})
	}
}

// 作用域守卫：OpenCode Go 仍使用原有结构化 403 计数策略，定制只改变 OpenAI。
func TestHandleUpstreamError_OpenCodeStructured403StillPenalizes(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &countingOpenAI403CounterCache{
		openAI403CounterCacheStub: openAI403CounterCacheStub{counts: []int64{1}},
	}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetOpenAI403CounterCache(counter)
	svc.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 505, Platform: PlatformOpenCodeGo, Type: AccountTypeAPIKey}

	require.True(t, svc.HandleUpstreamError(
		context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"message":"account forbidden"}}`),
	))
	require.Equal(t, 1, counter.increments)
	require.Equal(t, 1, repo.tempCalls)
}

// 作用域守卫：放行只针对 OpenAI 平台。其他平台的 403 处理不受影响。
func TestHandleUpstreamError_HTML403OnOtherPlatformsUnchanged(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			repo := &rateLimitAccountRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := &Account{ID: 506, Platform: platform, Type: AccountTypeAPIKey}

			shouldDisable := svc.HandleUpstreamError(
				context.Background(), account, http.StatusForbidden, http.Header{}, []byte(openAI403HTMLBody),
			)

			require.True(t, shouldDisable)
			require.Equal(t, 1, repo.setErrorCalls, "其他平台保持原有 SetError 行为")
		})
	}
}

func TestIsHTMLResponse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"doctype_lower", "<!doctype html><html></html>", true},
		{"doctype_upper", "<!DOCTYPE HTML>", true},
		{"bare_html", "<html lang=\"en\">", true},
		{"leading_whitespace", "\n\n   <html>", true},
		{"json_error", `{"error":{"message":"forbidden"}}`, false},
		{"plain_text", "Forbidden", false},
		{"empty", "", false},
		// XML/SVG 之类不是 HTML，不在放行范围内。
		{"xml_declaration", `<?xml version="1.0"?><error/>`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isHTMLResponse([]byte(tc.body)))
		})
	}
}
