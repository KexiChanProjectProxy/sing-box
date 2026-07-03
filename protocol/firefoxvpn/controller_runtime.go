package firefoxvpn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sagernet/sing-box/log"
)

func (c *AuthController) refreshLoop() {
	defer close(c.backgroundDone)
	var retryDelay time.Duration
	for {
		waitDuration := retryDelay
		if waitDuration == 0 {
			waitDuration = c.nextRefreshDelay()
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-c.backgroundCtx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
		if err := c.syncRuntimeState(c.backgroundCtx); err != nil {
			retryDelay = c.backoffDelay(0)
			c.logger.WarnEvent("firefoxvpn.controller.refresh", "runtime refresh failed", log.String("error", "refresh_failed"))
			continue
		}
		retryDelay = 0
	}
}

func (c *AuthController) nextRefreshDelay() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	delay := controllerBackgroundPollFloor
	if accessDelay, ok := c.delayUntilRefresh(c.accessToken, c.accessTokenExpiry, c.accessTokenRefreshMargin, now); ok {
		delay = accessDelay
	}
	if proxyDelay, ok := c.delayUntilProxyPassRefresh(now); ok && proxyDelay < delay {
		delay = proxyDelay
	}
	if delay < controllerBackgroundPollFloor {
		return controllerBackgroundPollFloor
	}
	return delay
}

func (c *AuthController) delayUntilRefresh(token string, expiresAt time.Time, margin time.Duration, now time.Time) (time.Duration, bool) {
	if token == "" || expiresAt.IsZero() {
		return 0, false
	}
	refreshAt := expiresAt.Add(-margin)
	if !refreshAt.After(now) {
		return 0, true
	}
	return refreshAt.Sub(now), true
}

func (c *AuthController) delayUntilProxyPassRefresh(now time.Time) (time.Duration, bool) {
	if c.proxyPass == nil {
		return 0, false
	}
	return c.delayUntilRefresh(c.proxyPass.Token, c.proxyPass.ExpiresAt(), c.proxyPassRefreshMargin, now)
}

func (c *AuthController) syncRuntimeState(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.withRetry(ctx, "controller sync", c.syncRuntimeStateOnce)
}

func (c *AuthController) syncRuntimeStateOnce(ctx context.Context) error {
	now := c.now()
	accessTokenNeedsRefresh := c.accessToken == "" || c.accessTokenExpiry.IsZero() || !c.accessTokenExpiry.Add(-c.accessTokenRefreshMargin).After(now)
	proxyPassNeedsRefresh := c.proxyPass == nil || !c.proxyPass.ExpiresAt().Add(-c.proxyPassRefreshMargin).After(now)
	if !accessTokenNeedsRefresh && !proxyPassNeedsRefresh {
		return nil
	}
	if accessTokenNeedsRefresh {
		if err := c.refreshOrLogin(ctx); err != nil {
			return err
		}
	}
	if proxyPassNeedsRefresh {
		proxyPass, err := c.fetchProxyPass(ctx)
		if err != nil {
			return err
		}
		c.proxyPass = proxyPass
	}
	return nil
}

func (c *AuthController) refreshOrLogin(ctx context.Context) error {
	if c.refreshToken != "" {
		refreshedToken, err := c.controlPlaneClient.RefreshOAuthToken(ctx, c.refreshToken)
		if err == nil {
			c.applyTokenResponse(refreshedToken)
			return nil
		}
		if !isRefreshTokenInvalid(err) {
			return fmt.Errorf("refresh access token: %w", err)
		}
		c.accessToken = ""
		c.accessTokenExpiry = time.Time{}
		c.refreshToken = ""
	}
	return c.loginAndExchange(ctx)
}

func (c *AuthController) loginAndExchange(ctx context.Context) error {
	loginResponse, err := c.controlPlaneClient.Login(ctx, c.email, c.password)
	if err != nil {
		return fmt.Errorf("login to Firefox Accounts: %w", err)
	}
	tokenResponse, err := c.controlPlaneClient.ExchangeOAuthToken(ctx, loginResponse.SessionToken)
	if err != nil {
		return fmt.Errorf("exchange Firefox OAuth token: %w", err)
	}
	c.applyTokenResponse(tokenResponse)
	return nil
}

func (c *AuthController) applyTokenResponse(tokenResponse *FxATokenResponse) {
	c.accessToken = tokenResponse.AccessToken
	c.refreshToken = tokenResponse.RefreshToken
	if tokenResponse.ExpiresIn > 0 {
		c.accessTokenExpiry = c.now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
		return
	}
	c.accessTokenExpiry = c.now()
}

func (c *AuthController) fetchProxyPass(ctx context.Context) (*ProxyPassInfo, error) {
	proxyPass, err := c.controlPlaneClient.FetchProxyPass(ctx, c.accessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch Guardian proxy pass: %w", err)
	}
	return proxyPass, nil
}

func (c *AuthController) withRetry(ctx context.Context, operation string, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		operationCtx, cancel := context.WithTimeout(ctx, c.operationTimeout)
		err := fn(operationCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == c.maxRetries {
			break
		}
		delay := c.backoffDelay(attempt)
		c.logger.WarnEvent("firefoxvpn.controller.retry", "controller retry", log.String("operation", operation), log.Int("attempt", attempt+1), log.Duration("backoff", delay), log.String("error", "retry_failed"))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("%s: %w", operation, ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("%s: %w", operation, lastErr)
}

func (c *AuthController) backoffDelay(attempt int) time.Duration {
	delay := c.retryBaseDelay << attempt
	if delay > c.retryMaxDelay {
		return c.retryMaxDelay
	}
	return delay
}

func isRefreshTokenInvalid(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid_grant") || strings.Contains(message, "http 400")
}
