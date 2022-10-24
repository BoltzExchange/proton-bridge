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

package bridge

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/gluon"
	imapEvents "github.com/ProtonMail/gluon/events"
	"github.com/ProtonMail/proton-bridge/v2/internal/async"
	"github.com/ProtonMail/proton-bridge/v2/internal/constants"
	"github.com/ProtonMail/proton-bridge/v2/internal/logging"
	"github.com/ProtonMail/proton-bridge/v2/internal/vault"
	"github.com/bradenaw/juniper/xsync"
	"github.com/sirupsen/logrus"
)

const (
	defaultClientName    = "UnknownClient"
	defaultClientVersion = "0.0.1"
)

func (bridge *Bridge) serveIMAP() error {
	imapListener, err := newListener(bridge.vault.GetIMAPPort(), bridge.vault.GetIMAPSSL(), bridge.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create IMAP listener: %w", err)
	}

	bridge.imapListener = imapListener

	if err := bridge.imapServer.Serve(context.Background(), bridge.imapListener); err != nil {
		return fmt.Errorf("failed to serve IMAP: %w", err)
	}

	if err := bridge.vault.SetIMAPPort(getPort(imapListener.Addr())); err != nil {
		return fmt.Errorf("failed to set IMAP port: %w", err)
	}

	return nil
}

func (bridge *Bridge) restartIMAP() error {
	if err := bridge.imapListener.Close(); err != nil {
		return fmt.Errorf("failed to close IMAP listener: %w", err)
	}

	return bridge.serveIMAP()
}

func (bridge *Bridge) closeIMAP(ctx context.Context) error {
	if err := bridge.imapServer.Close(ctx); err != nil {
		return fmt.Errorf("failed to close IMAP server: %w", err)
	}

	if bridge.imapListener != nil {
		if err := bridge.imapListener.Close(); err != nil {
			return fmt.Errorf("failed to close IMAP listener: %w", err)
		}
	}

	return nil
}

func (bridge *Bridge) handleIMAPEvent(event imapEvents.Event) {
	switch event := event.(type) {
	case imapEvents.SessionAdded:
		if !bridge.identifier.HasClient() {
			bridge.identifier.SetClient(defaultClientName, defaultClientVersion)
		}

	case imapEvents.IMAPID:
		bridge.identifier.SetClient(event.IMAPID.Name, event.IMAPID.Version)
	}
}

func getGluonDir(encVault *vault.Vault) (string, error) {
	empty, exists, err := isEmpty(encVault.GetGluonDir())
	if err != nil {
		return "", fmt.Errorf("failed to check if gluon dir is empty: %w", err)
	}

	if !exists {
		if err := os.MkdirAll(encVault.GetGluonDir(), 0700); err != nil {
			return "", fmt.Errorf("failed to create gluon dir: %w", err)
		}
	}

	if empty {
		if err := encVault.ForUser(func(user *vault.User) error {
			return user.ClearSyncStatus()
		}); err != nil {
			return "", fmt.Errorf("failed to reset user sync status: %w", err)
		}
	}

	return encVault.GetGluonDir(), nil
}

// nolint:funlen
func newIMAPServer(
	gluonDir string,
	version *semver.Version,
	tlsConfig *tls.Config,
	logClient, logServer bool,
	eventCh chan<- imapEvents.Event,
	tasks *xsync.Group,
) (*gluon.Server, error) {
	if logClient || logServer {
		log := logrus.WithField("protocol", "IMAP")
		log.Warning("================================================")
		log.Warning("THIS LOG WILL CONTAIN **DECRYPTED** MESSAGE DATA")
		log.Warning("================================================")
	}

	var imapClientLog io.Writer

	if logClient {
		imapClientLog = logging.NewIMAPLogger()
	} else {
		imapClientLog = io.Discard
	}

	var imapServerLog io.Writer

	if logServer {
		imapServerLog = logging.NewIMAPLogger()
	} else {
		imapServerLog = io.Discard
	}

	imapServer, err := gluon.New(
		gluon.WithTLS(tlsConfig),
		gluon.WithDataDir(gluonDir),
		gluon.WithVersionInfo(
			int(version.Major()),
			int(version.Minor()),
			int(version.Patch()),
			constants.FullAppName,
			"TODO",
			"TODO",
		),
		gluon.WithLogger(
			imapClientLog,
			imapServerLog,
		),
	)
	if err != nil {
		return nil, err
	}

	tasks.Once(func(ctx context.Context) {
		async.ForwardContext(ctx, eventCh, imapServer.AddWatcher())
	})

	tasks.Once(func(ctx context.Context) {
		async.RangeContext(ctx, imapServer.GetErrorCh(), func(err error) {
			logrus.WithError(err).Error("IMAP server error")
		})
	})

	return imapServer, nil
}

// isEmpty returns whether the given directory is empty.
// If the directory does not exist, the second return value is false.
func isEmpty(dir string) (bool, bool, error) {
	if _, err := os.Stat(dir); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, false, fmt.Errorf("failed to stat %s: %w", dir, err)
		}

		return true, false, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, false, fmt.Errorf("failed to read dir %s: %w", dir, err)
	}

	return len(entries) == 0, true, nil
}