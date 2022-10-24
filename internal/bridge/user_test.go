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

package bridge_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ProtonMail/proton-bridge/v2/internal/bridge"
	"github.com/ProtonMail/proton-bridge/v2/internal/events"
	"github.com/ProtonMail/proton-bridge/v2/internal/vault"
	"github.com/stretchr/testify/require"
	"gitlab.protontech.ch/go/liteapi"
	"gitlab.protontech.ch/go/liteapi/server"
)

func TestBridge_WithoutUsers(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			require.Empty(t, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))
		})

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			require.Empty(t, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_Login(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID, err := bridge.LoginFull(ctx, username, password, nil, nil)
			require.NoError(t, err)

			// The user is now connected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginLogoutLogin(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID := must(bridge.LoginFull(ctx, username, password, nil, nil))

			// The user is now connected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))

			// Logout the user.
			require.NoError(t, bridge.LogoutUser(ctx, userID))

			// The user is now disconnected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))

			// Login the user again.
			newUserID := must(bridge.LoginFull(ctx, username, password, nil, nil))
			require.Equal(t, userID, newUserID)

			// The user is connected again.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginDeleteLogin(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID := must(bridge.LoginFull(ctx, username, password, nil, nil))

			// The user is now connected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))

			// Delete the user.
			require.NoError(t, bridge.DeleteUser(ctx, userID))

			// The user is now gone.
			require.Empty(t, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))

			// Login the user again.
			newUserID := must(bridge.LoginFull(ctx, username, password, nil, nil))
			require.Equal(t, userID, newUserID)

			// The user is connected again.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginDeauthLogin(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID := must(bridge.LoginFull(ctx, username, password, nil, nil))

			// Get a channel to receive the deauth event.
			eventCh, done := bridge.GetEvents(events.UserDeauth{})
			defer done()

			// Deauth the user.
			require.NoError(t, s.RevokeUser(userID))

			// The user is eventually disconnected.
			require.Eventually(t, func() bool {
				return len(getConnectedUserIDs(t, bridge)) == 0
			}, 10*time.Second, time.Second)

			// We should get a deauth event.
			require.IsType(t, events.UserDeauth{}, <-eventCh)

			// Login the user after the disconnection.
			newUserID := must(bridge.LoginFull(ctx, username, password, nil, nil))
			require.Equal(t, userID, newUserID)

			// The user is connected again.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginExpireLogin(t *testing.T) {
	const authLife = 2 * time.Second

	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		s.SetAuthLife(authLife)

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user. Its auth will only be valid for a short time.
			userID := must(bridge.LoginFull(ctx, username, password, nil, nil))

			// Wait until the auth expires.
			time.Sleep(authLife)

			// The user will have to refresh but the logout will still succeed.
			require.NoError(t, bridge.LogoutUser(ctx, userID))
		})
	})
}

func TestBridge_FailToLoad(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		// Login the user.
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))
		})

		// Deauth the user while bridge is stopped.
		require.NoError(t, s.RevokeUser(userID))

		// When bridge starts, the user will not be logged in.
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoadWithoutInternet(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		// Login the user.
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))
		})

		// Simulate loss of internet connection.
		netCtl.Disable()

		// Start bridge without internet.
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Initially, users are not connected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))

			time.Sleep(5 * time.Second)

			// Simulate internet connection.
			netCtl.Enable()

			// The user will eventually be connected.
			require.Eventually(t, func() bool {
				return len(getConnectedUserIDs(t, bridge)) == 1 && getConnectedUserIDs(t, bridge)[0] == userID
			}, 10*time.Second, time.Second)
		})
	})
}

func TestBridge_LoginRestart(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))
		})

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginLogoutRestart(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))

			// Logout the user.
			require.NoError(t, bridge.LogoutUser(ctx, userID))
		})

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// The user is still disconnected.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_LoginDeleteRestart(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))

			// Delete the user.
			require.NoError(t, bridge.DeleteUser(ctx, userID))
		})

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// The user is still gone.
			require.Empty(t, bridge.GetUserIDs())
			require.Empty(t, getConnectedUserIDs(t, bridge))
		})
	})
}

func TestBridge_FailLoginRecover(t *testing.T) {
	for i := uint64(1); i < 10; i++ {
		t.Run(fmt.Sprintf("read %v%% of the data", 100*i/10), func(t *testing.T) {
			withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
				var userID string

				// Log the user in, wait for it to sync, then log it out.
				// (We don't want to count message sync data in the test.)
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					syncCh, done := chToType[events.Event, events.SyncFinished](bridge.GetEvents(events.SyncFinished{}))
					defer done()

					userID = must(bridge.LoginFull(ctx, username, password, nil, nil))
					require.Equal(t, userID, (<-syncCh).UserID)
					require.NoError(t, bridge.LogoutUser(ctx, userID))
				})

				var total uint64

				// Now that the user is synced, we can measure exactly how much data is needed during login.
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					total = countBytesRead(netCtl, func() {
						must(bridge.LoginFull(ctx, username, password, nil, nil))
					})

					require.NoError(t, bridge.LogoutUser(ctx, userID))
				})

				// Now simulate failing to login.
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					// Simulate a partial read.
					netCtl.SetReadLimit(i * total / 10)

					// We should fail to log the user in because we can't fully read its data.
					require.Error(t, getErr(bridge.LoginFull(ctx, username, password, nil, nil)))

					// The user should still be there (but disconnected).
					require.Equal(t, []string{userID}, bridge.GetUserIDs())
					require.Empty(t, getConnectedUserIDs(t, bridge))
				})

				// Simulate the network recovering.
				netCtl.SetReadLimit(0)

				// We should now be able to log the user in.
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					require.NoError(t, getErr(bridge.LoginFull(ctx, username, password, nil, nil)))

					// The user should be there, now connected.
					require.Equal(t, []string{userID}, bridge.GetUserIDs())
					require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
				})
			})
		})
	}
}

func TestBridge_FailLoadRecover(t *testing.T) {
	for i := uint64(1); i < 10; i++ {
		t.Run(fmt.Sprintf("read %v%% of the data", 100*i/10), func(t *testing.T) {
			withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
				var userID string

				// Log the user in and wait for it to sync.
				// (We don't want to count message sync data in the test.)
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					syncCh, done := chToType[events.Event, events.SyncFinished](bridge.GetEvents(events.SyncFinished{}))
					defer done()

					userID = must(bridge.LoginFull(ctx, username, password, nil, nil))
					require.Equal(t, userID, (<-syncCh).UserID)
				})

				// See how much data it takes to load the user at startup.
				total := countBytesRead(netCtl, func() {
					withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
						// ...
					})
				})

				// Simulate a partial read.
				netCtl.SetReadLimit(i * total / 10)

				// We should fail to load the user; it should be listed but disconnected.
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					require.Equal(t, []string{userID}, bridge.GetUserIDs())
					require.Empty(t, getConnectedUserIDs(t, bridge))
				})

				// Simulate the network recovering.
				netCtl.SetReadLimit(0)

				// We should now be able to load the user.
				withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
					require.Equal(t, []string{userID}, bridge.GetUserIDs())
					require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))
				})
			})
		})
	}
}

func TestBridge_BridgePass(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		var userID string

		var pass []byte

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID = must(bridge.LoginFull(ctx, username, password, nil, nil))

			// Retrieve the bridge pass.
			pass = must(bridge.GetUserInfo(userID)).BridgePass

			// Log the user out.
			require.NoError(t, bridge.LogoutUser(ctx, userID))

			// Log the user back in.
			must(bridge.LoginFull(ctx, username, password, nil, nil))

			// The bridge pass should be the same.
			require.Equal(t, pass, pass)
		})

		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// The bridge should load the user.
			require.Equal(t, []string{userID}, bridge.GetUserIDs())
			require.Equal(t, []string{userID}, getConnectedUserIDs(t, bridge))

			// The bridge pass should be the same.
			require.Equal(t, pass, must(bridge.GetUserInfo(userID)).BridgePass)
		})
	})
}

func TestBridge_AddressMode(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			// Login the user.
			userID, err := bridge.LoginFull(ctx, username, password, nil, nil)
			require.NoError(t, err)

			// Get the user's info.
			info, err := bridge.GetUserInfo(userID)
			require.NoError(t, err)

			// The user is in combined mode by default.
			require.Equal(t, vault.CombinedMode, info.AddressMode)

			// Put the user in split mode.
			require.NoError(t, bridge.SetAddressMode(ctx, userID, vault.SplitMode))

			// Get the user's info.
			info, err = bridge.GetUserInfo(userID)
			require.NoError(t, err)

			// The user is in split mode.
			require.Equal(t, vault.SplitMode, info.AddressMode)
		})
	})
}

func TestBridge_LoginLogoutRepeated(t *testing.T) {
	withEnv(t, func(ctx context.Context, s *server.Server, netCtl *liteapi.NetCtl, locator bridge.Locator, storeKey []byte) {
		withBridge(ctx, t, s.GetHostURL(), netCtl, locator, storeKey, func(bridge *bridge.Bridge, mocks *bridge.Mocks) {
			for i := 0; i < 10; i++ {
				// Log the user in.
				userID := must(bridge.LoginFull(ctx, username, password, nil, nil))

				// Log the user out.
				require.NoError(t, bridge.LogoutUser(ctx, userID))
			}
		})
	})
}

// getErr returns the error that was passed to it.
func getErr[T any](val T, err error) error {
	return err
}