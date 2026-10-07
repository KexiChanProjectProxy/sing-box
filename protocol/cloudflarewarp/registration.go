//go:build with_quic

package cloudflarewarp

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"net/http"
	"os"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/warpapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

// Replaced in tests.
var (
	apiBaseURL        = warpapi.DefaultBaseURL
	testAPIHTTPClient *http.Client
)

func (o *Outbound) apiClient() (*warpapi.Client, error) {
	o.apiAccess.Lock()
	defer o.apiAccess.Unlock()
	if o.apiHTTP == nil {
		if testAPIHTTPClient != nil {
			o.apiHTTP = testAPIHTTPClient
		} else {
			dialerOptions := o.options.DialerOptions
			if o.options.APIDetour != "" {
				dialerOptions = option.DialerOptions{Detour: o.options.APIDetour}
			}
			apiDialer, err := dialer.NewWithOptions(dialer.Options{
				Context:          o.ctx,
				Options:          dialerOptions,
				RemoteIsDomain:   true,
				ResolverOnDetour: true,
			})
			if err != nil {
				return nil, E.Cause(err, "create API dialer")
			}
			o.apiHTTP = warpapi.NewHTTPClient(o.ctx, apiDialer)
		}
	}
	return warpapi.NewClient(o.apiHTTP, apiBaseURL), nil
}

// loadRegistration returns the registration to connect with, registering a
// new device in automatic mode when the cache holds none.
func (o *Outbound) loadRegistration(licenseDone *bool) (*warpapi.Registration, *ecdsa.PrivateKey, error) {
	o.access.Lock()
	registration, privateKey := o.registration, o.privateKey
	o.access.Unlock()
	if registration != nil {
		return registration, privateKey, nil
	}
	var err error
	if o.static != nil {
		registration = o.static
		if !*licenseDone && o.options.License != "" {
			*licenseDone = true
			ctx, cancel := context.WithTimeout(o.ctx, registrationTimeout)
			var client *warpapi.Client
			client, err = o.apiClient()
			if err == nil {
				_, err = client.ApplyLicense(ctx, registration.DeviceID, registration.AccessToken, o.options.License)
			}
			cancel()
			if err != nil {
				o.logger.WarnEvent("warp.license.error", "failed to apply license", log.Err(err))
			}
		}
	} else {
		registration, err = o.loadCachedRegistration()
		if err != nil {
			return nil, nil, err
		}
	}
	privateKey, err = warpapi.ParsePrivateKey(registration.PrivateKey)
	if err != nil {
		return nil, nil, err
	}
	err = o.device.UpdateAddresses(registration.Addresses())
	if err != nil {
		return nil, nil, err
	}
	o.access.Lock()
	o.registration, o.privateKey = registration, privateKey
	o.access.Unlock()
	return registration, privateKey, nil
}

func (o *Outbound) loadCachedRegistration() (*warpapi.Registration, error) {
	tag := o.Tag()
	var registration *warpapi.Registration
	data, err := o.cacheFile.LoadCloudflareWARPRegistration(tag)
	if err == nil {
		var cached warpapi.Registration
		err = json.Unmarshal(data, &cached)
		if err == nil && cached.Version == warpapi.RegistrationVersion && cached.PrivateKey != "" && len(cached.Addresses()) > 0 {
			registration = &cached
		} else {
			o.logger.WarnEvent("warp.register", "ignoring unusable cached registration")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, E.Cause(err, "load cached registration")
	}
	ctx, cancel := context.WithTimeout(o.ctx, registrationTimeout)
	defer cancel()
	client, err := o.apiClient()
	if err != nil {
		return nil, err
	}
	changed := false
	if registration == nil {
		privateKey, err := warpapi.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		registration, err = client.Register(ctx, privateKey, warpapi.RegisterOptions{
			Name:      o.options.DeviceName,
			AccessJWT: o.options.AccessJWT,
			License:   o.options.License,
		})
		if err != nil {
			return nil, err
		}
		changed = true
		o.logger.InfoEvent("warp.register", "registered a new WARP device", log.String("device_id", registration.DeviceID))
	} else if o.options.License != "" && o.options.License != registration.License {
		_, err = client.ApplyLicense(ctx, registration.DeviceID, registration.AccessToken, o.options.License)
		if err != nil {
			o.logger.WarnEvent("warp.license.error", "failed to apply license", log.Err(err))
		} else {
			registration.License = o.options.License
			changed = true
		}
	}
	if changed {
		data, err = json.Marshal(registration)
		if err != nil {
			return nil, err
		}
		err = o.cacheFile.StoreCloudflareWARPRegistration(tag, data)
		if err != nil {
			return nil, E.Cause(err, "store registration")
		}
	}
	return registration, nil
}

// resetRegistration drops the in-memory and cached registration so the next
// attempt registers a new device.
func (o *Outbound) resetRegistration() {
	o.access.Lock()
	o.registration, o.privateKey = nil, nil
	o.access.Unlock()
	if o.cacheFile != nil {
		err := o.cacheFile.DeleteCloudflareWARPRegistration(o.Tag())
		if err != nil {
			o.logger.WarnEvent("warp.register", "failed to delete cached registration", log.Err(err))
		}
	}
}
