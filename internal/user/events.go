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

package user

import (
	"context"
	"fmt"

	"github.com/ProtonMail/gluon/imap"
	"github.com/ProtonMail/gluon/queue"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/proton-bridge/v2/internal/events"
	"github.com/ProtonMail/proton-bridge/v2/internal/vault"
	"github.com/bradenaw/juniper/xslices"
	"gitlab.protontech.ch/go/liteapi"
)

// handleAPIEvent handles the given liteapi.Event.
func (user *User) handleAPIEvent(ctx context.Context, event liteapi.Event) error {
	if event.User != nil {
		if err := user.handleUserEvent(ctx, *event.User); err != nil {
			return err
		}
	}

	if len(event.Addresses) > 0 {
		if err := user.handleAddressEvents(ctx, event.Addresses); err != nil {
			return err
		}
	}

	if len(event.Labels) > 0 {
		if err := user.handleLabelEvents(ctx, event.Labels); err != nil {
			return err
		}
	}

	if len(event.Messages) > 0 {
		if err := user.handleMessageEvents(ctx, event.Messages); err != nil {
			return err
		}
	}

	return nil
}

// handleUserEvent handles the given user event.
func (user *User) handleUserEvent(_ context.Context, userEvent liteapi.User) error {
	user.apiUser.Save(userEvent)

	user.eventCh.Enqueue(events.UserChanged{
		UserID: user.ID(),
	})

	return nil
}

// handleAddressEvents handles the given address events.
// GODT-1945: If split address mode, need to signal back to bridge to update the addresses.
func (user *User) handleAddressEvents(ctx context.Context, addressEvents []liteapi.AddressEvent) error {
	for _, event := range addressEvents {
		switch event.Action {
		case liteapi.EventCreate:
			if err := user.handleCreateAddressEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle create address event: %w", err)
			}

		case liteapi.EventUpdate:
			if err := user.handleUpdateAddressEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle update address event: %w", err)
			}

		case liteapi.EventDelete:
			if err := user.handleDeleteAddressEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to delete address: %w", err)
			}

		case liteapi.EventUpdateFlags:
			user.log.Warn("Not implemented yet.")
		}
	}

	return nil
}

func (user *User) handleCreateAddressEvent(ctx context.Context, event liteapi.AddressEvent) error {
	if had := user.apiAddrs.Set(event.Address.ID, event.Address); had {
		return fmt.Errorf("address %q already exists", event.Address.ID)
	}

	switch user.vault.AddressMode() {
	case vault.CombinedMode:
		user.apiAddrs.Index(0, func(addrID string, _ liteapi.Address) {
			user.updateCh.SetFrom(event.Address.ID, addrID)
		})

	case vault.SplitMode:
		user.updateCh.Set(event.Address.ID, queue.NewQueuedChannel[imap.Update](0, 0))
	}

	user.eventCh.Enqueue(events.UserAddressCreated{
		UserID:    user.ID(),
		AddressID: event.Address.ID,
		Email:     event.Address.Email,
	})

	if user.vault.AddressMode() == vault.SplitMode {
		if ok, err := user.updateCh.GetErr(event.Address.ID, func(updateCh *queue.QueuedChannel[imap.Update]) error {
			return syncLabels(ctx, user.client, updateCh)
		}); !ok {
			return fmt.Errorf("no such address %q", event.Address.ID)
		} else if err != nil {
			return fmt.Errorf("failed to sync labels to new address: %w", err)
		}
	}

	return nil
}

func (user *User) handleUpdateAddressEvent(_ context.Context, event liteapi.AddressEvent) error { //nolint:unparam
	if had := user.apiAddrs.Set(event.Address.ID, event.Address); !had {
		return fmt.Errorf("address %q does not exist", event.Address.ID)
	}

	user.eventCh.Enqueue(events.UserAddressUpdated{
		UserID:    user.ID(),
		AddressID: event.Address.ID,
		Email:     event.Address.Email,
	})

	return nil
}

func (user *User) handleDeleteAddressEvent(_ context.Context, event liteapi.AddressEvent) error {
	var email string

	if ok := user.apiAddrs.GetDelete(event.ID, func(apiAddr liteapi.Address) {
		email = apiAddr.Email
	}); !ok {
		return fmt.Errorf("no such address %q", event.ID)
	}

	if ok := user.updateCh.GetDelete(event.ID, func(updateCh *queue.QueuedChannel[imap.Update]) {
		if user.vault.AddressMode() == vault.SplitMode {
			updateCh.CloseAndDiscardQueued()
		}
	}); !ok {
		return fmt.Errorf("no such address %q", event.ID)
	}

	user.eventCh.Enqueue(events.UserAddressDeleted{
		UserID:    user.ID(),
		AddressID: event.ID,
		Email:     email,
	})

	return nil
}

// handleLabelEvents handles the given label events.
func (user *User) handleLabelEvents(ctx context.Context, labelEvents []liteapi.LabelEvent) error {
	for _, event := range labelEvents {
		switch event.Action {
		case liteapi.EventCreate:
			if err := user.handleCreateLabelEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle create label event: %w", err)
			}

		case liteapi.EventUpdate, liteapi.EventUpdateFlags:
			if err := user.handleUpdateLabelEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle update label event: %w", err)
			}

		case liteapi.EventDelete:
			if err := user.handleDeleteLabelEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle delete label event: %w", err)
			}
		}
	}

	return nil
}

func (user *User) handleCreateLabelEvent(_ context.Context, event liteapi.LabelEvent) error { //nolint:unparam
	user.updateCh.IterValues(func(updateCh *queue.QueuedChannel[imap.Update]) {
		updateCh.Enqueue(newMailboxCreatedUpdate(imap.MailboxID(event.ID), getMailboxName(event.Label)))
	})

	return nil
}

func (user *User) handleUpdateLabelEvent(_ context.Context, event liteapi.LabelEvent) error { //nolint:unparam
	user.updateCh.IterValues(func(updateCh *queue.QueuedChannel[imap.Update]) {
		updateCh.Enqueue(imap.NewMailboxUpdated(imap.MailboxID(event.ID), getMailboxName(event.Label)))
	})

	return nil
}

func (user *User) handleDeleteLabelEvent(_ context.Context, event liteapi.LabelEvent) error { //nolint:unparam
	user.updateCh.IterValues(func(updateCh *queue.QueuedChannel[imap.Update]) {
		updateCh.Enqueue(imap.NewMailboxDeleted(imap.MailboxID(event.ID)))
	})

	return nil
}

// handleMessageEvents handles the given message events.
func (user *User) handleMessageEvents(ctx context.Context, messageEvents []liteapi.MessageEvent) error {
	for _, event := range messageEvents {
		switch event.Action {
		case liteapi.EventCreate:
			if err := user.handleCreateMessageEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle create message event: %w", err)
			}

		case liteapi.EventUpdate, liteapi.EventUpdateFlags:
			if err := user.handleUpdateMessageEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to handle update message event: %w", err)
			}

		case liteapi.EventDelete:
			return ErrNotImplemented
		}
	}

	return nil
}

func (user *User) handleCreateMessageEvent(ctx context.Context, event liteapi.MessageEvent) error {
	full, err := user.client.GetFullMessage(ctx, event.Message.ID)
	if err != nil {
		return fmt.Errorf("failed to get full message: %w", err)
	}

	return user.withAddrKR(event.Message.AddressID, func(_, addrKR *crypto.KeyRing) error {
		buildRes, err := buildRFC822(full, addrKR)
		if err != nil {
			return fmt.Errorf("failed to build RFC822 message: %w", err)
		}

		user.updateCh.Get(full.AddressID, func(updateCh *queue.QueuedChannel[imap.Update]) {
			updateCh.Enqueue(imap.NewMessagesCreated(buildRes.update))
		})

		return nil
	})
}

func (user *User) handleUpdateMessageEvent(_ context.Context, event liteapi.MessageEvent) error { //nolint:unparam
	update := imap.NewMessageMailboxesUpdated(
		imap.MessageID(event.ID),
		mapTo[string, imap.MailboxID](xslices.Filter(event.Message.LabelIDs, wantLabelID)),
		event.Message.Seen(),
		event.Message.Starred(),
	)

	user.updateCh.Get(event.Message.AddressID, func(updateCh *queue.QueuedChannel[imap.Update]) {
		updateCh.Enqueue(update)
	})

	return nil
}

func getMailboxName(label liteapi.Label) []string {
	var name []string

	switch label.Type {
	case liteapi.LabelTypeFolder:
		name = append([]string{folderPrefix}, label.Path...)

	case liteapi.LabelTypeLabel:
		name = append([]string{labelPrefix}, label.Path...)

	case liteapi.LabelTypeContactGroup:
		fallthrough
	case liteapi.LabelTypeSystem:
		fallthrough
	default:
		name = label.Path
	}

	return name
}