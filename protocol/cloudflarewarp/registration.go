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
	} else if o.options.Ephemeral {
		registration, err = o.registerEphemeral()
		if err != nil {
			return nil, nil, err
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
		registration, err = o.register(ctx, client)
		if err != nil {
			return nil, err
		}
		changed = true
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

func (o *Outbound) register(ctx context.Context, client *warpapi.Client) (*warpapi.Registration, error) {
	privateKey, err := warpapi.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	registration, err := client.Register(ctx, privateKey, warpapi.RegisterOptions{
		Name:      o.options.DeviceName,
		AccessJWT: o.options.AccessJWT,
		License:   o.options.License,
	})
	if err != nil {
		return nil, err
	}
	o.logger.InfoEvent("warp.register", "registered a new WARP device", log.String("device_id", registration.DeviceID), log.Bool("ephemeral", o.options.Ephemeral))
	return registration, nil
}

// registerEphemeral registers a device that lives only in memory and is
// deleted again when the outbound closes.
func (o *Outbound) registerEphemeral() (*warpapi.Registration, error) {
	client, err := o.apiClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(o.ctx, registrationTimeout)
	defer cancel()
	return o.register(ctx, client)
}

// deleteEphemeral removes an ephemeral device from Cloudflare, best effort.
func (o *Outbound) deleteEphemeral(registration *warpapi.Registration) {
	if registration == nil || !o.options.Ephemeral {
		return
	}
	client, err := o.apiClient()
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), ephemeralDeleteTimeout)
		err = client.DeleteDevice(ctx, registration.DeviceID, registration.AccessToken)
		cancel()
	}
	if err != nil {
		o.logger.WarnEvent("warp.unregister.error", "failed to delete the ephemeral WARP device", log.String("device_id", registration.DeviceID), log.Err(err))
		return
	}
	o.logger.InfoEvent("warp.unregister", "deleted the ephemeral WARP device", log.String("device_id", registration.DeviceID))
}

// resetRegistration drops the in-memory and cached registration so the next
// attempt registers a new device.
func (o *Outbound) resetRegistration() {
	o.access.Lock()
	registration := o.registration
	o.registration, o.privateKey = nil, nil
	o.access.Unlock()
	o.deleteEphemeral(registration)
	if o.cacheFile != nil {
		err := o.cacheFile.DeleteCloudflareWARPRegistration(o.Tag())
		if err != nil {
			o.logger.WarnEvent("warp.register", "failed to delete cached registration", log.Err(err))
		}
	}
}
