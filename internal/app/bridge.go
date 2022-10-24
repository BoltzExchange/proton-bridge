// Copyright (c) 2022 Proton AG
//
// This file is part of Proton Mail Bridge.
//
// Proton Mail Bridge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Proton Mail Bridge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Proton Mail Bridge.  If not, see <https://www.gnu.org/licenses/>.

package app

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/go-autostart"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/proton-bridge/v2/internal/bridge"
	"github.com/ProtonMail/proton-bridge/v2/internal/constants"
	"github.com/ProtonMail/proton-bridge/v2/internal/dialer"
	"github.com/ProtonMail/proton-bridge/v2/internal/events"
	"github.com/ProtonMail/proton-bridge/v2/internal/locations"
	"github.com/ProtonMail/proton-bridge/v2/internal/sentry"
	"github.com/ProtonMail/proton-bridge/v2/internal/updater"
	"github.com/ProtonMail/proton-bridge/v2/internal/useragent"
	"github.com/ProtonMail/proton-bridge/v2/internal/vault"
	"github.com/ProtonMail/proton-bridge/v2/internal/versioner"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const vaultSecretName = "bridge-vault-key"

// deleteOldGoIMAPFiles Set with `-ldflags -X app.deleteOldGoIMAPFiles=true` to enable cleanup of old imap cache data.
var deleteOldGoIMAPFiles bool //nolint:gochecknoglobals

// withBridge creates creates and tears down the bridge.
func withBridge( //nolint:funlen
	c *cli.Context,
	exe string,
	locations *locations.Locations,
	version *semver.Version,
	identifier *useragent.UserAgent,
	_ *sentry.Reporter,
	vault *vault.Vault,
	cookieJar http.CookieJar,
	fn func(*bridge.Bridge, <-chan events.Event) error,
) error {
	// Create the underlying dialer used by the bridge.
	// It only connects to trusted servers and reports any untrusted servers it finds.
	pinningDialer := dialer.NewPinningTLSDialer(
		dialer.NewBasicTLSDialer(constants.APIHost),
		dialer.NewTLSReporter(constants.APIHost, constants.AppVersion(version.Original()), identifier, dialer.TrustedAPIPins),
		dialer.NewTLSPinChecker(dialer.TrustedAPIPins),
	)

	// Delete old go-imap cache files
	if deleteOldGoIMAPFiles {
		if err := locations.CleanGoIMAPCache(); err != nil {
			logrus.WithError(err).Error("Failed to remove old go-imap cache")
		}
	}

	// Create a proxy dialer which switches to a proxy if the request fails.
	proxyDialer := dialer.NewProxyTLSDialer(pinningDialer, constants.APIHost)

	// Create the autostarter.
	autostarter := newAutostarter(exe)

	// Create the update installer.
	updater, err := newUpdater(locations)
	if err != nil {
		return fmt.Errorf("could not create updater: %w", err)
	}

	// Create a new bridge.
	bridge, eventCh, err := bridge.New(
		// The app stuff.
		locations,
		vault,
		autostarter,
		updater,
		version,

		// The API stuff.
		constants.APIHost,
		cookieJar,
		identifier,
		pinningDialer,
		dialer.CreateTransportWithDialer(proxyDialer),
		proxyDialer,

		// The logging stuff.
		c.String(flagLogIMAP) == "client" || c.String(flagLogIMAP) == "all",
		c.String(flagLogIMAP) == "server" || c.String(flagLogIMAP) == "all",
		c.Bool(flagLogSMTP),
	)
	if err != nil {
		return fmt.Errorf("could not create bridge: %w", err)
	}

	// Close the bridge when we exit.
	defer func() {
		if err := bridge.Close(c.Context); err != nil {
			logrus.WithError(err).Error("Failed to close bridge")
		}
	}()

	return fn(bridge, eventCh)
}

func newAutostarter(exe string) *autostart.App {
	return &autostart.App{
		Name:        constants.FullAppName,
		DisplayName: constants.FullAppName,
		Exec:        []string{exe, "--" + flagNoWindow},
	}
}

func newUpdater(locations *locations.Locations) (*updater.Updater, error) {
	updatesDir, err := locations.ProvideUpdatesPath()
	if err != nil {
		return nil, fmt.Errorf("could not provide updates path: %w", err)
	}

	key, err := crypto.NewKeyFromArmored(updater.DefaultPublicKey)
	if err != nil {
		return nil, fmt.Errorf("could not create key from armored: %w", err)
	}

	verifier, err := crypto.NewKeyRing(key)
	if err != nil {
		return nil, fmt.Errorf("could not create key ring: %w", err)
	}

	return updater.NewUpdater(
		updater.NewInstaller(versioner.New(updatesDir)),
		verifier,
		constants.UpdateName,
		runtime.GOOS,
	), nil
}