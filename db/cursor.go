package db

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const orderKey = rune('o')
const countriesKey = rune('c')
const pageKey = rune('p')
const eskKey = rune('e')
const limitKey = rune('l')

const divider = rune('|')
const arrSep = rune(',')
const colon = rune(':')

func NewCursor(opts GetListOpts) (Cursor, error) {
	sb := strings.Builder{}

	keys := []rune{
		orderKey,
		countriesKey,
		pageKey,
		eskKey,
		limitKey,
	}

	var err error
	for _, k := range keys {
		switch k {
		case orderKey:
			if len(string(opts.Order)) > 0 {
				err = writeKV(&sb, writeRune(k), writeString(string(opts.Order)))
				_, err = sb.WriteRune(divider)
				if err != nil {
					return Cursor{}, fmt.Errorf("could not write divider for cursor: %w", err)
				}
			}
		case countriesKey:
			if len(opts.Countries) > 0 {
				err = writeKV(&sb, writeRune(k), writeStringSlice(opts.Countries))
				_, err = sb.WriteRune(divider)
				if err != nil {
					return Cursor{}, fmt.Errorf("could not write divider for cursor: %w", err)
				}
			}
		case pageKey:
			if opts.Page > 0 {
				err = writeKV(&sb, writeRune(k), writeInt(opts.Page))
				_, err = sb.WriteRune(divider)
				if err != nil {
					return Cursor{}, fmt.Errorf("could not write divider for cursor: %w", err)
				}
			}
		case eskKey:
			if len(opts.ExclStartKey) > 0 {
				err = writeKV(&sb, writeRune(k), writeString(opts.ExclStartKey))
				_, err = sb.WriteRune(divider)
				if err != nil {
					return Cursor{}, fmt.Errorf("could not write divider for cursor: %w", err)
				}
			}
		case limitKey:
			if opts.Limit > 0 {
				err = writeKV(&sb, writeRune(k), writeInt(opts.Limit))
				_, err = sb.WriteRune(divider)
				if err != nil {
					return Cursor{}, fmt.Errorf("could not write divider for cursor: %w", err)
				}
			}
		}

		if err != nil {
			return Cursor{}, fmt.Errorf("could not write cursor for key %s: %w", string(k), err)
		}
	}

	str := sb.String()

	str = strings.TrimRightFunc(str, func(r rune) bool { return r == divider })
	str = strings.TrimLeftFunc(str, func(r rune) bool { return r == divider })

	return Cursor{
		str:  str,
		opts: opts,
	}, nil
}

func writeIf(fn func() error, cond bool) error {
	if cond {
		return fn()
	}
	return nil
}

func writeKV(sb *strings.Builder, kWriter sbWriter, vWriter sbWriter) error {
	err := kWriter(sb)
	if err != nil {
		return err
	}
	sb.WriteRune(colon)
	err = vWriter(sb)
	return err
}

type sbWriter func(*strings.Builder) error

func writeRune(r rune) sbWriter {
	return func(b *strings.Builder) error {
		_, err := b.WriteRune(r)
		return err
	}
}

func writeString(s string) sbWriter {
	return func(b *strings.Builder) error {
		_, err := b.WriteString(s)
		return err
	}
}

func writeInt(i int) sbWriter {
	return func(b *strings.Builder) error {
		_, err := b.WriteString(strconv.Itoa(i))
		return err
	}
}

func writeStringSlice(ss []string) sbWriter {
	return func(b *strings.Builder) error {
		for i, s := range ss {
			_, err := b.WriteString(s)
			if err != nil {
				return err
			}

			if i < len(ss)-1 {
				b.WriteRune(arrSep)
			}
		}
		return nil
	}

}

func ParseCursor(cursorStr string) (*Cursor, error) {
	cursor := Cursor{}
	err := cursor.Parse(cursorStr)
	return &cursor, err
}

func MustParseCursor(cursorStr string) *Cursor {
	c, err := ParseCursor(cursorStr)
	if err != nil {
		panic(fmt.Sprintf("must parse cursor %s: %s", cursorStr, err.Error()))
	}
	return c
}

type Cursor struct {
	Debug  bool
	opts   GetListOpts
	str    string
	reader *bytes.Reader
}

func (c *Cursor) Opts() GetListOpts {
	return c.opts
}

func (c *Cursor) String() string {
	return c.str
}

func (c *Cursor) EncodedString() string {
	return base64.URLEncoding.EncodeToString([]byte(c.str))
}

func (c *Cursor) Parse(cursorStr string) error {
	c.debugf("received cursor for parsing: %s\n", cursorStr)

	dec, err := base64.URLEncoding.DecodeString(cursorStr)
	if err != nil {
		c.debugf("cursor is not b64 assuming decoded: %s\n", cursorStr)

		c.str = cursorStr
	} else {
		c.str = string(dec)
	}

	c.opts = GetListOpts{}
	c.reader = bytes.NewReader([]byte(c.str))

	c.debugf("beginning parse: %s\n", c.str)

	for {
		b, err := c.reader.ReadByte()
		if err == io.EOF {
			c.debugln("EOF")
			break
		}
		if err != nil && err != io.EOF {
			c.debugln(err.Error())
			return err
		}

		c.debugln(string(b))

		switch b {
		case byte(divider):
			// for dividers we can keep going, next byte should be a key
			continue
		case byte(orderKey):
			err = c.checkReadRune(colon)
			if err != nil {
				err = fmt.Errorf("could not read order from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}

			c.opts.Order, err = c.readOrder()
			if err != nil {
				err = fmt.Errorf("could not read order from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
		case byte(countriesKey):
			err = c.checkReadRune(colon)
			if err != nil {
				err = fmt.Errorf("could not read countries from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}

			c.opts.Countries, err = c.readCountries()
			if err != nil {
				err = fmt.Errorf("could not read countries from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}

		case byte(pageKey):
			err = c.checkReadRune(colon)
			if err != nil {
				err = fmt.Errorf("could not read page from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}

			c.opts.Page, err = c.readInt()
			if err != nil {
				err = fmt.Errorf("could not read page from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
		case byte(eskKey):
			err = c.checkReadRune(colon)
			if err != nil {
				err = fmt.Errorf("could not read excl start key from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
			c.opts.ExclStartKey, err = c.readString()
			if err != nil {
				err = fmt.Errorf("could not read excl start key from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
		case byte(limitKey):
			err = c.checkReadRune(colon)
			if err != nil {
				err = fmt.Errorf("could not read limit from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
			c.opts.Limit, err = c.readInt()
			if err != nil {
				err = fmt.Errorf("could not read excl start key from cursor: %w", err)
				c.debugln(err.Error())
				return err
			}
		}
	}

	return nil
}

func (c *Cursor) checkReadRune(r rune) error {
	c.debugf("checkReadRune: '%s'\n", string(r))
	ch, _, err := c.reader.ReadRune()
	if err != nil {
		return err
	}
	if ch != r {
		return fmt.Errorf("expected rune %s got %s", string(ch), string(r))
	}
	return nil
}

func (c *Cursor) readOrder() (Order, error) {
	buf := make([]byte, 4)
	_, err := c.reader.Read(buf[:3])
	if err != nil {
		return "", err
	}

	if string(buf[:3]) == string(ASC) {
		return ASC, nil
	}

	_, err = c.reader.Read(buf[3:])
	if err != nil {
		return "", nil
	}

	if string(buf) == string(DESC) {
		return DESC, nil
	}

	return "", fmt.Errorf("invalid order: %s", string(buf))

}

func (c *Cursor) readCountries() ([]string, error) {
	c.debugln("readCountries")

	countries := []string{}
	currCountry := []byte{}

	var err error
	dividerFound := false
	for {
		b, berr := c.reader.ReadByte()
		if berr != nil && berr != io.EOF {
			c.debugln(berr.Error())
			err = berr
			break
		}

		if berr == io.EOF {
			c.debugf("appending country: %s\n", string(currCountry))
			countries = append(countries, string(currCountry))
			err = berr
			break
		}

		switch rune(b) {
		case arrSep:
			c.debugf("appending country: %s\n", string(currCountry))

			countries = append(countries, string(currCountry))
			currCountry = []byte{}
		case divider:
			c.debugf("appending country: %s\n", string(currCountry))
			countries = append(countries, string(currCountry))
			dividerFound = true
		default:
			currCountry = append(currCountry, b)
		}

		if dividerFound {
			c.debugln("divider found")
			break
		}
	}

	if err != nil && err != io.EOF {
		c.debugln(err.Error())

		return nil, err
	}

	c.debugf("countries: %v\n", countries)
	return countries, nil
}

func (c *Cursor) readInt() (int, error) {
	intBytes := []byte{}

	var err error
	for {
		b, berr := c.reader.ReadByte()
		if berr != nil {
			err = berr
			break
		}
		if b == byte(divider) {
			break
		}

		intBytes = append(intBytes, b)
	}

	if err != nil && err != io.EOF {
		return 0, err
	}

	return strconv.Atoi(string(intBytes))
}

func (c *Cursor) readString() (string, error) {
	stringBytes := []byte{}

	var err error
	for {
		b, berr := c.reader.ReadByte()
		if berr != nil {
			err = berr
			break
		}
		if b == byte(divider) {
			break
		}

		stringBytes = append(stringBytes, b)
	}

	if err != nil && err != io.EOF {
		return "", err
	}

	return string(stringBytes), nil
}

func (c *Cursor) debugln(msg string) {
	if c.Debug {
		fmt.Println(msg)
	}
}

func (c *Cursor) debugf(msg string, args ...any) {
	if c.Debug {
		fmt.Printf(msg, args...)
	}
}
