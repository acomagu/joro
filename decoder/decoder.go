package decoder

import (
	"io"
	"sort"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/pkg/errors"
)

type Decoder struct {
	*scanner.Scanner
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		Scanner: new(scanner.Scanner).Init(r),
	}
}

func (d *Decoder) Decode() (interface{}, error) {
	v, err := d.decodeToMapOrV()
	if err != nil {
		return nil, err
	}

	return d.makeKeyStringOrCvtSlice(v)
}

func (d *Decoder) makeKeyStringOrCvtSlice(v interface{}) (interface{}, error) {
	m, ok := v.(map[interface{}]interface{})
	if !ok {
		return v, nil
	}

	if sm, ok := d.tryMakeKeyString(m); ok {
		for k, vv := range sm {
			nv, err := d.makeKeyStringOrCvtSlice(vv)
			if err != nil {
				return nil, err
			}

			sm[k] = nv
		}

		return sm, nil
	}
	if sl, ok := d.tryCvtSlice(m); ok {
		for i, vv := range sl {
			nv, err := d.makeKeyStringOrCvtSlice(vv)
			if err != nil {
				return nil, err
			}

			sl[i] = nv
		}

		return sl, nil
	}

	return nil, errors.New("invalid map")
}

func (d *Decoder) tryCvtSlice(m map[interface{}]interface{}) ([]interface{}, bool) {
	var keys []int64
	for k := range m {
		ik, ok := k.(int64)
		if !ok {
			return nil, false
		}

		keys = append(keys, ik)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	var r []interface{}
	for _, k := range keys {
		r = append(r, m[k])
	}

	return r, true
}

func (d *Decoder) tryMakeKeyString(m map[interface{}]interface{}) (map[string]interface{}, bool) {
	r := make(map[string]interface{})
	for k, v := range m {
		sk, ok := k.(string)
		if !ok {
			return nil, false
		}

		r[sk] = v
	}

	return r, true
}

func (d *Decoder) decodeToMapOrV() (interface{}, error) {
	var paths []interface{}
	for d.Peek() != scanner.EOF {
		d.skipBrank()
		path, err := d.decodeLine()
		if err != nil {
			return nil, err
		}

		paths = append(paths, path)
	}

	var kvs []kv
	for _, path := range paths {
		if s, ok := path.(*kv); ok {
			kvs = append(kvs, *s)
		} else {
			return path, nil
		}
	}

	return d.kvsToMap(kvs)
}

func (d *Decoder) kvsToMap(kvs []kv) (map[interface{}]interface{}, error) {
	m := make(map[interface{}]interface{})

	// create map[interface{}][]kv
	for _, s := range kvs {
		if ss, ok := s.value.(*kv); ok {
			if _, ok := m[s.key]; ok {
				skvs, ok := m[s.key].([]kv)
				if !ok {
					panic("not kvs")
				}

				m[s.key] = append(skvs, *ss)
			} else {
				m[s.key] = []kv{*ss}
			}
		} else {
			m[s.key] = s.value
		}
	}

	// extract each []kv to map
	for k, v := range m {
		kvs, ok := v.([]kv)
		if !ok {
			continue
		}

		mm, err := d.kvsToMap(kvs)
		if err != nil {
			return nil, err
		}

		m[k] = mm
	}

	return m, nil
}

type kv struct {
	key   interface{}
	value interface{}
}

func (d *Decoder) decodeLine() (interface{}, error) {
	var v interface{}
	var err error
	switch d.Peek() {
	case '<':
		v, err = d.scanBracket()
	case '#':
		v, err = d.scanArrIndex()
	default:
		v, err = d.scanStr()
	}
	if err != nil {
		return nil, err
	}

	d.skipBrank()
	if d.Peek() == '\n' {
		d.Next()
		return v, nil
	}

	deeper, err := d.decodeLine()
	if err != nil {
		return nil, err
	}

	return &kv{
		key:   v,
		value: deeper,
	}, nil
}

func (d *Decoder) isBrank(r rune) bool {
	return r == ' '
}

func (d *Decoder) skipBrank() {
	for d.isBrank(d.Peek()) {
		d.Next()
	}
}

func (d *Decoder) isStrChar(r rune) bool {
	return !d.isBrank(r) && r != '\n' && r != scanner.EOF
}

func (d *Decoder) scanStr() (string, error) {
	var s string
	for ; d.isStrChar(d.Peek()); d.Next() {
		if d.Peek() == '\\' {
			d.Next()
			if d.Peek() == 'n' {
				s += "\n"
				continue
			}
		}

		s += string(d.Peek())
	}

	return s, nil
}

func (d *Decoder) scanArrIndex() (interface{}, error) {
	d.Next() // skip `#`

	return d.scanInt()
}

func (d *Decoder) isIntChar(r rune) bool {
	_, err := strconv.Atoi(string(r))
	return err == nil
}

func (d *Decoder) scanInt() (int64, error) {
	var s string
	for d.isIntChar(d.Peek()) {
		s += string(d.Peek())
		d.Next()
	}

	return strconv.ParseInt(s, 10, 64)
}

func (d *Decoder) scanBracket() (interface{}, error) {
	d.Next() // skip `<`

	var v interface{}
	var err error
	switch {
	case d.isNumChar(d.Peek()):
		v, err = d.scanNum()
	case d.isKwdChar(d.Peek()):
		v, err = d.scanKwd()
	case d.Peek() == '>':
		v = ""
	default:
		return nil, errors.New("invalid bracket value")
	}
	if err != nil {
		return nil, err
	}

	if d.Peek() != '>' {
		return nil, errors.Errorf("unexpected '%c'", d.Peek())
	}

	d.Next()

	return v, nil
}

func (d *Decoder) isKwdChar(r rune) bool {
	return strings.ContainsRune("truefalsenull", r)
}

func (d *Decoder) scanKwd() (interface{}, error) {
	var s string
	for d.isKwdChar(d.Peek()) {
		s += string(d.Peek())
		d.Next()
	}

	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}

	return nil, errors.Errorf("could not parse bracket value: %s", s)
}

func (d *Decoder) isNumChar(r rune) bool {
	if strings.ContainsRune(".e+-", r) {
		return true
	}

	_, err := strconv.Atoi(string(r))
	return err == nil
}

func (d *Decoder) scanNum() (float64, error) {
	var s string
	for d.isNumChar(d.Peek()) {
		s += string(d.Peek())
		d.Next()
	}

	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "could not parse %s as float64", s)
	}

	return n, nil
}
