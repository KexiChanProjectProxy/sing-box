package firefoxvpn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

const (
	defaultControllerTimeout           = 30 * time.Second
	defaultControllerRetryBaseDelay    = 500 * time.Millisecond
	defaultControllerRetryMaxDelay     = 8 * time.Second
	defaultControllerRetryCount        = 5
	defaultControllerAccessTokenMargin = time.Minute
	defaultControllerProxyPassMargin   = time.Minute
	controllerBackgroundPollFloor      = time.Second
)

type ControlPlaneClientFactory func(ctx context.Context, apiDetour string) (*ControlPlaneClient, error)

type AuthController struct {
	logger             log.StructuredLogger
	controlPlaneClient *ControlPlaneClient
	email              string
	password           string

	mu                sync.Mutex
	accessToken       string
	accessTokenExpiry time.Time
	refreshToken      string
	proxyPass         *ProxyPassInfo
	started           bool

	now                      func() time.Time
	operationTimeout         time.Duration
	retryBaseDelay           time.Duration
	retryMaxDelay            time.Duration
	maxRetries               int
	accessTokenRefreshMargin time.Duration
	proxyPassRefreshMargin   time.Duration

	backgroundCtx    context.Context
	backgroundCancel context.CancelFunc
	backgroundDone   chan struct{}
}

func NewAuthController(ctx context.Context, options option.FirefoxVPNOutboundOptions, factory ControlPlaneClientFactory) (*AuthController, error) {
	return newAuthControllerWithLogger(ctx, log.NewNOPFactory().Logger(), options, factory)
}

func newAuthControllerWithLogger(ctx context.Context, logger log.StructuredLogger, options option.FirefoxVPNOutboundOptions, factory ControlPlaneClientFactory) (*AuthController, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if factory == nil {
		factory = NewControlPlaneClient
	}
	controlPlaneClient, err := factory(ctx, options.APIDetour)
	if err != nil {
		return nil, fmt.Errorf("create control-plane client: %w", err)
	}
	if logger == nil {
		logger = log.NewNOPFactory().Logger()
	}
	backgroundCtx, backgroundCancel := context.WithCancel(ctx)
	return &AuthController{
		logger:                   logger,
		controlPlaneClient:       controlPlaneClient,
		email:                    options.Email,
		password:                 options.Password,
		now:                      time.Now,
		operationTimeout:         defaultControllerTimeout,
		retryBaseDelay:           defaultControllerRetryBaseDelay,
		retryMaxDelay:            defaultControllerRetryMaxDelay,
		maxRetries:               defaultControllerRetryCount,
		accessTokenRefreshMargin: defaultControllerAccessTokenMargin,
		proxyPassRefreshMargin:   defaultControllerProxyPassMargin,
		backgroundCtx:            backgroundCtx,
		backgroundCancel:         backgroundCancel,
		backgroundDone:           make(chan struct{}),
	}, nil
}

func (c *AuthController) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.syncRuntimeState(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	c.started = true
	go c.refreshLoop()
	return nil
}

func (c *AuthController) GetProxyPass(ctx context.Context) (*ProxyPassInfo, error) {
	if err := c.syncRuntimeState(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proxyPass == nil {
		return nil, fmt.Errorf("proxy pass unavailable")
	}
	proxyPassCopy := *c.proxyPass
	return &proxyPassCopy, nil
}

func (c *AuthController) Close() error {
	c.mu.Lock()
	started := c.started
	c.started = false
	c.mu.Unlock()
	c.backgroundCancel()
	if started {
		select {
		case <-c.backgroundDone:
		default:
			<-c.backgroundDone
		}
	}
	c.mu.Lock()
	c.password = ""
	c.accessToken = ""
	c.accessTokenExpiry = time.Time{}
	c.refreshToken = ""
	c.proxyPass = nil
	c.mu.Unlock()
	return nil
}
