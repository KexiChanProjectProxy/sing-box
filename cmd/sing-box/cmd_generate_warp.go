package main

import (
	"context"
	"net/http"
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/sing-box/common/warpapi"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
)

var (
	warpLicense   string
	warpAccessJWT string
	warpName      string
	warpModel     string
	warpLocale    string
	warpTag       string
	warpIPv6      bool
	warpRaw       bool
	warpTimeout   time.Duration
)

var commandGenerateWARPRegistration = &cobra.Command{
	Use:   "warp-registration",
	Short: "Register a Cloudflare WARP (MASQUE) device and print a cloudflare-warp outbound",
	Long: "Register a new Cloudflare WARP device for the MASQUE protocol and print a ready-to-use " +
		"cloudflare-warp outbound. Registering accepts Cloudflare's Terms of Service " +
		"(https://www.cloudflare.com/application/terms/). The output contains secrets.",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateWARPRegistration()
		if err != nil {
			log.FatalEvent("cli.error", err.Error(), log.Err(err))
		}
	},
}

func init() {
	flags := commandGenerateWARPRegistration.Flags()
	flags.StringVar(&warpLicense, "license", "", "WARP+ license key to bind to the device")
	flags.StringVar(&warpAccessJWT, "access-jwt", "", "Zero Trust team token (CF-Access-Jwt-Assertion)")
	flags.StringVar(&warpName, "name", "", "device name shown in the Cloudflare dashboard")
	flags.StringVar(&warpModel, "model", warpapi.DefaultModel, "device model")
	flags.StringVar(&warpLocale, "locale", warpapi.DefaultLocale, "device locale")
	flags.StringVar(&warpTag, "tag", "warp", "outbound tag")
	flags.BoolVar(&warpIPv6, "ipv6", false, "use the IPv6 endpoint as server")
	flags.BoolVar(&warpRaw, "raw", false, "print the raw registration instead of an outbound")
	flags.DurationVar(&warpTimeout, "timeout", 30*time.Second, "request timeout")
	commandGenerate.AddCommand(commandGenerateWARPRegistration)
}

type warpOutboundSnippet struct {
	Type              string   `json:"type"`
	Tag               string   `json:"tag"`
	Server            string   `json:"server"`
	ServerPort        uint16   `json:"server_port"`
	PrivateKey        string   `json:"private_key"`
	Address           []string `json:"address"`
	EndpointPublicKey string   `json:"endpoint_public_key,omitempty"`
	DeviceID          string   `json:"device_id"`
	AccessToken       string   `json:"access_token"`
}

func generateWARPRegistration() error {
	ctx, cancel := context.WithTimeout(context.Background(), warpTimeout)
	defer cancel()
	key, err := warpapi.GeneratePrivateKey()
	if err != nil {
		return err
	}
	os.Stderr.WriteString("Registering accepts Cloudflare's Terms of Service: https://www.cloudflare.com/application/terms/\n")
	client := warpapi.NewClient(&http.Client{Timeout: warpTimeout}, warpapi.DefaultBaseURL)
	registration, err := client.Register(ctx, key, warpapi.RegisterOptions{
		Model:     warpModel,
		Locale:    warpLocale,
		Name:      warpName,
		AccessJWT: warpAccessJWT,
		License:   warpLicense,
	})
	if err != nil {
		return err
	}
	var output any
	if warpRaw {
		output = registration
	} else {
		var server netip.Addr
		if warpIPv6 {
			server = registration.EndpointV6
		} else {
			server = registration.EndpointV4
		}
		if !server.IsValid() {
			return E.New("registration returned no endpoint for the selected address family")
		}
		addresses := make([]string, 0, 2)
		for _, prefix := range registration.Addresses() {
			addresses = append(addresses, prefix.String())
		}
		output = warpOutboundSnippet{
			Type:              C.TypeCloudflareWARP,
			Tag:               warpTag,
			Server:            server.String(),
			ServerPort:        warpapi.DefaultPort,
			PrivateKey:        registration.PrivateKey,
			Address:           addresses,
			EndpointPublicKey: registration.EndpointPublicKey,
			DeviceID:          registration.DeviceID,
			AccessToken:       registration.AccessToken,
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
