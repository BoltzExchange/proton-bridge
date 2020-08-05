// Copyright (c) 2020 Proton Technologies AG
//
// This file is part of ProtonMail Bridge.
//
// ProtonMail Bridge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// ProtonMail Bridge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with ProtonMail Bridge.  If not, see <https://www.gnu.org/licenses/>.

package message

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"strings"

	"github.com/ProtonMail/proton-bridge/pkg/message/parser"
	"github.com/ProtonMail/proton-bridge/pkg/pmapi"
	"github.com/emersion/go-message"
	"github.com/jaytaylor/html2text"
)

func Parse(r io.Reader, key, keyName string) (m *pmapi.Message, mimeMessage, plainBody string, attReaders []io.Reader, err error) {
	p, err := parser.New(r)
	if err != nil {
		return
	}

	m = pmapi.NewMessage()

	if err = parseHeader(m, p.Root().Header); err != nil {
		return
	}

	if m.Attachments, attReaders, err = collectAttachments(p); err != nil {
		return
	}

	if m.Body, plainBody, err = buildBodies(p); err != nil {
		return
	}

	if m.MIMEType, err = determineMIMEType(p); err != nil {
		return
	}

	if key != "" {
		attachPublicKey(p.Root(), key, keyName)
	}

	if mimeMessage, err = writeMIMEMessage(p); err != nil {
		return
	}

	return
}

func collectAttachments(p *parser.Parser) (atts []*pmapi.Attachment, data []io.Reader, err error) {
	w := p.NewWalker().
		RegisterContentDispositionHandler("attachment", func(p *parser.Part) error {
			att, err := parseAttachment(p.Header)
			if err != nil {
				return err
			}

			atts = append(atts, att)
			data = append(data, bytes.NewReader(p.Body))

			return nil
		}).
		RegisterContentTypeHandler("text/calendar", func(p *parser.Part) error {
			att, err := parseAttachment(p.Header)
			if err != nil {
				return err
			}

			atts = append(atts, att)
			data = append(data, bytes.NewReader(p.Body))

			return nil
		}).
		RegisterContentTypeHandler("text/.*", func(p *parser.Part) error {
			return nil
		}).
		RegisterDefaultHandler(func(p *parser.Part) error {
			if len(p.Children()) > 0 {
				return nil
			}

			att, err := parseAttachment(p.Header)
			if err != nil {
				return err
			}

			atts = append(atts, att)
			data = append(data, bytes.NewReader(p.Body))

			return nil
		})

	if err = w.Walk(); err != nil {
		return
	}

	return
}

func buildBodies(p *parser.Parser) (richBody, plainBody string, err error) {
	richParts, err := collectBodyParts(p, "text/html")
	if err != nil {
		return
	}

	plainParts, err := collectBodyParts(p, "text/plain")
	if err != nil {
		return
	}

	if len(richParts) != len(plainParts) {
		return "", "", errors.New("unequal number of rich and plain parts")
	}

	richBuilder, plainBuilder := strings.Builder{}, strings.Builder{}

	for i := 0; i < len(richParts); i++ {
		_, _ = richBuilder.Write(richParts[i].Body)
		_, _ = plainBuilder.Write(getPlainBody(plainParts[i]))
	}

	return richBuilder.String(), plainBuilder.String(), nil
}

// collectBodyParts collects all body parts in the parse tree, preferring
// parts of the given content type if alternatives exist.
func collectBodyParts(p *parser.Parser, preferredContentType string) (parser.Parts, error) {
	v := p.
		NewVisitor(func(p *parser.Part, visit parser.Visit) (interface{}, error) {
			childParts, err := collectChildParts(p, visit)
			if err != nil {
				return nil, err
			}

			return joinChildParts(childParts), nil
		}).
		RegisterRule("multipart/alternative", func(p *parser.Part, visit parser.Visit) (interface{}, error) {
			childParts, err := collectChildParts(p, visit)
			if err != nil {
				return nil, err
			}

			return bestChoice(childParts, preferredContentType)
		}).
		RegisterRule("text/plain", func(p *parser.Part, visit parser.Visit) (interface{}, error) {
			return parser.Parts{p}, nil
		}).
		RegisterRule("text/html", func(p *parser.Part, visit parser.Visit) (interface{}, error) {
			return parser.Parts{p}, nil
		})

	res, err := v.Visit()
	if err != nil {
		return nil, err
	}

	return res.(parser.Parts), nil
}

func collectChildParts(p *parser.Part, visit parser.Visit) ([]parser.Parts, error) {
	childParts := []parser.Parts{}

	for _, child := range p.Children() {
		res, err := visit(child)
		if err != nil {
			return nil, err
		}

		childParts = append(childParts, res.(parser.Parts))
	}

	return childParts, nil
}

func joinChildParts(childParts []parser.Parts) parser.Parts {
	res := parser.Parts{}

	for _, parts := range childParts {
		res = append(res, parts...)
	}

	return res
}

func bestChoice(childParts []parser.Parts, preferredContentType string) (parser.Parts, error) {
	// If one of the parts has preferred content type, use that.
	for i := len(childParts) - 1; i >= 0; i-- {
		if allPartsHaveContentType(childParts[i], preferredContentType) {
			return childParts[i], nil
		}
	}

	// Otherwise, choose the last one.
	return childParts[len(childParts)-1], nil
}

func allPartsHaveContentType(parts parser.Parts, contentType string) bool {
	for _, part := range parts {
		t, _, err := part.Header.ContentType()
		if err != nil {
			return false
		}

		if t != contentType {
			return false
		}
	}

	return true
}

func determineMIMEType(p *parser.Parser) (string, error) {
	var isHTML bool

	w := p.NewWalker().
		RegisterContentTypeHandler("text/html", func(p *parser.Part) (err error) {
			isHTML = true
			return
		})

	if err := w.Walk(); err != nil {
		return "", err
	}

	if isHTML {
		return "text/html", nil
	}

	return "text/plain", nil
}

func getPlainBody(part *parser.Part) []byte {
	contentType, _, err := part.Header.ContentType()
	if err != nil {
		return part.Body
	}

	switch contentType {
	case "text/plain":
		return part.Body

	case "text/html":
		text, err := html2text.FromReader(bytes.NewReader(part.Body))
		if err != nil {
			return part.Body
		}

		return []byte(text)

	default:
		return part.Body
	}
}

func writeMIMEMessage(p *parser.Parser) (string, error) {
	buf := new(bytes.Buffer)

	if err := p.NewWriter().Write(buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func attachPublicKey(p *parser.Part, key, keyName string) {
	h := message.Header{}

	h.Set("Content-Type", fmt.Sprintf(`application/pgp-key; name="%v"`, keyName))
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%v.asc.pgp"`, keyName))
	h.Set("Content-Transfer-Encoding", "base64")

	// TODO: Split body at col width 72.

	p.AddChild(&parser.Part{
		Header: h,
		Body:   []byte(key),
	})
}

func parseHeader(m *pmapi.Message, h message.Header) error {
	m.Header = make(mail.Header)

	fields := h.Fields()

	for fields.Next() {
		text, err := fields.Text()
		if err != nil {
			return err
		}

		// TODO: Is this okay? Might need to append/split/something.
		m.Header[fields.Key()] = []string{text}

		switch strings.ToLower(fields.Key()) {
		case "subject":
			m.Subject = text

		case "from":
			sender, err := mail.ParseAddress(text)
			if err != nil {
				return err
			}
			m.Sender = sender

		case "to":
			toList, err := mail.ParseAddressList(text)
			if err != nil {
				return err
			}
			m.ToList = toList

		case "reply-to":
			replyTos, err := mail.ParseAddressList(text)
			if err != nil {
				return err
			}
			m.ReplyTos = replyTos

		case "cc":
			ccList, err := mail.ParseAddressList(text)
			if err != nil {
				return err
			}
			m.CCList = ccList

		case "bcc":
			bccList, err := mail.ParseAddressList(text)
			if err != nil {
				return err
			}
			m.BCCList = bccList

		case "date":
			date, err := mail.ParseDate(text)
			if err != nil {
				return err
			}
			m.Time = date.Unix()
		}
	}

	return nil
}

func parseAttachment(h message.Header) (att *pmapi.Attachment, err error) {
	att = &pmapi.Attachment{}

	if att.MIMEType, _, err = h.ContentType(); err != nil {
		return
	}

	if _, dispParams, dispErr := h.ContentDisposition(); dispErr != nil {
		var ext []string

		if ext, err = mime.ExtensionsByType(att.MIMEType); err != nil {
			return
		}

		if len(ext) > 0 {
			att.Name = "attachment" + ext[0]
		}
	} else {
		att.Name = dispParams["filename"]
	}

	att.ContentID = strings.Trim(h.Get("Content-Id"), " <>")

	// TODO: Set att.Header

	return
}
