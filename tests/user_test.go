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

package tests

import (
	"context"
	"errors"
	"fmt"

	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/xslices"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"gitlab.protontech.ch/go/liteapi"
	"golang.org/x/exp/slices"
)

func (s *scenario) thereExistsAnAccountWithUsernameAndPassword(username, password string) error {
	// Create the user.
	userID, addrID, err := s.t.api.CreateUser(username, username, []byte(password))
	if err != nil {
		return err
	}

	// Set the ID of this user.
	s.t.setUserID(username, userID)

	// Set the password of this user.
	s.t.setUserPass(userID, password)

	// Set the address of this user (right now just the same as the username, but let's stay flexible).
	s.t.setUserAddr(userID, addrID, username)

	return nil
}

func (s *scenario) theAccountHasAdditionalAddress(username, address string) error {
	userID := s.t.getUserID(username)

	addrID, err := s.t.api.CreateAddress(userID, address, []byte(s.t.getUserPass(userID)))
	if err != nil {
		return err
	}

	s.t.setUserAddr(userID, addrID, address)

	return nil
}

func (s *scenario) theAccountNoLongerHasAdditionalAddress(username, address string) error {
	userID := s.t.getUserID(username)
	addrID := s.t.getUserAddrID(userID, address)

	if err := s.t.api.RemoveAddress(userID, addrID); err != nil {
		return err
	}

	s.t.unsetUserAddr(userID, addrID)

	return nil
}

func (s *scenario) theAccountHasCustomFolders(username string, count int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return s.t.withClient(ctx, username, func(ctx context.Context, client *liteapi.Client) error {
		for idx := 0; idx < count; idx++ {
			if _, err := client.CreateLabel(ctx, liteapi.CreateLabelReq{
				Name: uuid.NewString(),
				Type: liteapi.LabelTypeFolder,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *scenario) theAccountHasCustomLabels(username string, count int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return s.t.withClient(ctx, username, func(ctx context.Context, client *liteapi.Client) error {
		for idx := 0; idx < count; idx++ {
			if _, err := client.CreateLabel(ctx, liteapi.CreateLabelReq{
				Name: uuid.NewString(),
				Type: liteapi.LabelTypeLabel,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *scenario) theAccountHasTheFollowingCustomMailboxes(username string, table *godog.Table) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type CustomMailbox struct {
		Name string `bdd:"name"`
		Type string `bdd:"type"`
	}

	wantMailboxes, err := unmarshalTable[CustomMailbox](table)
	if err != nil {
		return err
	}

	return s.t.withClient(ctx, username, func(ctx context.Context, client *liteapi.Client) error {
		for _, wantMailbox := range wantMailboxes {
			var labelType liteapi.LabelType

			switch wantMailbox.Type {
			case "folder":
				labelType = liteapi.LabelTypeFolder

			case "label":
				labelType = liteapi.LabelTypeLabel
			}

			if _, err := client.CreateLabel(ctx, liteapi.CreateLabelReq{
				Name: wantMailbox.Name,
				Type: labelType,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *scenario) theAddressOfAccountHasTheFollowingMessagesInMailbox(address, username, mailbox string, table *godog.Table) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userID := s.t.getUserID(username)
	addrID := s.t.getUserAddrID(userID, address)
	mboxID := s.t.getMBoxID(userID, mailbox)

	wantMessages, err := unmarshalTable[Message](table)
	if err != nil {
		return err
	}

	return s.t.createMessages(ctx, username, addrID, xslices.Map(wantMessages, func(message Message) liteapi.ImportReq {
		return liteapi.ImportReq{
			Metadata: liteapi.ImportMetadata{
				AddressID: addrID,
				LabelIDs:  []string{mboxID},
				Unread:    liteapi.Bool(message.Unread),
				Flags:     liteapi.MessageFlagReceived,
			},
			Message: message.Build(),
		}
	}))
}

func (s *scenario) theAddressOfAccountHasMessagesInMailbox(address, username string, count int, mailbox string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userID := s.t.getUserID(username)
	addrID := s.t.getUserAddrID(userID, address)
	mboxID := s.t.getMBoxID(userID, mailbox)

	return s.t.createMessages(ctx, username, addrID, iterator.Collect(iterator.Map(iterator.Counter(count), func(idx int) liteapi.ImportReq {
		return liteapi.ImportReq{
			Metadata: liteapi.ImportMetadata{
				AddressID: addrID,
				LabelIDs:  []string{mboxID},
				Flags:     liteapi.MessageFlagReceived,
			},
			Message: Message{
				Subject: fmt.Sprintf("%d", idx),
				To:      fmt.Sprintf("%d@pm.me", idx),
				From:    fmt.Sprintf("%d@pm.me", idx),
				Body:    fmt.Sprintf("body %d", idx),
			}.Build(),
		}
	})))
}

func (s *scenario) userLogsInWithUsernameAndPassword(username, password string) error {
	userID, err := s.t.bridge.LoginFull(context.Background(), username, []byte(password), nil, nil)
	if err != nil {
		s.t.pushError(err)
	} else {
		if userID != s.t.getUserID(username) {
			return errors.New("user ID mismatch")
		}

		info, err := s.t.bridge.GetUserInfo(userID)
		if err != nil {
			return err
		}

		s.t.setUserBridgePass(userID, info.BridgePass)
	}

	return nil
}

func (s *scenario) userLogsOut(username string) error {
	return s.t.bridge.LogoutUser(context.Background(), s.t.getUserID(username))
}

func (s *scenario) userIsDeleted(username string) error {
	return s.t.bridge.DeleteUser(context.Background(), s.t.getUserID(username))
}

func (s *scenario) theAuthOfUserIsRevoked(username string) error {
	return s.t.api.RevokeUser(s.t.getUserID(username))
}

func (s *scenario) userIsListedAndConnected(username string) error {
	user, err := s.t.bridge.GetUserInfo(s.t.getUserID(username))
	if err != nil {
		return err
	}

	if user.Username != username {
		return errors.New("user not listed")
	}

	if !user.Connected {
		return errors.New("user not connected")
	}

	return nil
}

func (s *scenario) userIsEventuallyListedAndConnected(username string) error {
	return eventually(func() error {
		return s.userIsListedAndConnected(username)
	})
}

func (s *scenario) userIsListedButNotConnected(username string) error {
	user, err := s.t.bridge.GetUserInfo(s.t.getUserID(username))
	if err != nil {
		return err
	}

	if user.Username != username {
		return errors.New("user not listed")
	}

	if user.Connected {
		return errors.New("user connected")
	}

	return nil
}

func (s *scenario) userIsNotListed(username string) error {
	if slices.Contains(s.t.bridge.GetUserIDs(), s.t.getUserID(username)) {
		return errors.New("user listed")
	}

	return nil
}

func (s *scenario) userFinishesSyncing(username string) error {
	return s.bridgeSendsSyncStartedAndFinishedEventsForUser(username)
}