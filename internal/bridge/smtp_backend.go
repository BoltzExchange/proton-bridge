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
	"fmt"
	"io"

	"github.com/ProtonMail/proton-bridge/v2/internal/safe"
	"github.com/ProtonMail/proton-bridge/v2/internal/user"
	"github.com/emersion/go-smtp"
)

type smtpBackend struct {
	users *safe.Map[string, *user.User]
}

type smtpSession struct {
	users *safe.Map[string, *user.User]

	userID string
	authID string

	from string
	to   []string
}

func (be *smtpBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &smtpSession{
		users: be.users,
	}, nil
}

func (s *smtpSession) AuthPlain(username, password string) error {
	return s.users.ValuesErr(func(users []*user.User) error {
		for _, user := range users {
			addrID, err := user.CheckAuth(username, []byte(password))
			if err != nil {
				continue
			}

			s.userID = user.ID()
			s.authID = addrID

			return nil
		}

		return fmt.Errorf("invalid username or password")
	})
}

func (s *smtpSession) Reset() {
	s.from = ""
	s.to = nil
}

func (s *smtpSession) Logout() error {
	s.Reset()
	return nil
}

func (s *smtpSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *smtpSession) Rcpt(to string) error {
	if len(to) > 0 {
		s.to = append(s.to, to)
	}

	return nil
}

func (s *smtpSession) Data(r io.Reader) error {
	if ok, err := s.users.GetErr(s.userID, func(user *user.User) error {
		return user.SendMail(s.authID, s.from, s.to, r)
	}); !ok {
		return fmt.Errorf("no such user %q", s.userID)
	} else if err != nil {
		return fmt.Errorf("failed to send mail: %w", err)
	}

	return nil
}