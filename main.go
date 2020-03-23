package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/acomagu/go-ufml"
)

func main() {
	p, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var dat interface{}
	switch p.input {
	case jsonFormat:
		var err error
		if err = json.NewDecoder(os.Stdin).Decode(&dat); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

	case joroFormat:
		if err := ufml.NewDecoder(os.Stdin).Decode(&dat); err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "\nhint: table format cannot be unjoro.")
			os.Exit(1)
		}

	default:
		panic("invalid params.input value")
	}

	switch p.output {
	case jsonFormat:
		if err := json.NewEncoder(os.Stdout).Encode(dat); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

	case joroFormat:
		var rows [][]string
		if p.isTable {
			rows = toTableRows(dat)

			sw := toStrWithWidthTable(rows)
			for _, row := range sw {
				for ic, col := range row {
					s := col.s
					if p.hasPadding {
						s = putMargin(s, col.w)
					}

					if ic > 0 {
						fmt.Printf("%c", p.delimiter) // delimiter
					}
					fmt.Print(s)
				}
				fmt.Println()
			}
		} else {
			if err := ufml.NewEncoder(os.Stdout).Encode(dat); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}

	default:
		panic("invalid params.output value")
	}
}

type format int8

const (
	jsonFormat format = iota + 1
	joroFormat
)

type params struct {
	input format

	output     format
	isTable    bool
	hasPadding bool
	delimiter  rune
}

func parseFlags(args []string) (*params, error) {
	var isTable, unjoro, noPadding, padding, fromJoro, fromJSON bool
	var delimiterStr string
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.BoolVar(&isTable, "table", false, "convert to table format")
	fs.BoolVar(&isTable, "t", false, "convert to table format (shorthand)")
	fs.BoolVar(&unjoro, "v", false, "convert from Joro rows to JSON")
	fs.BoolVar(&noPadding, "no-padding", false, "without space padding")
	fs.BoolVar(&noPadding, "p", false, "without space padding (shorthand)")
	fs.BoolVar(&padding, "padding", false, "with space padding (default)")
	fs.BoolVar(&fromJoro, "from-joro", false, "parse input as Joro rows instead of JSON")
	fs.BoolVar(&fromJoro, "j", false, "parse input as Joro rows instead of JSON (shorthand)")
	fs.BoolVar(&fromJSON, "from-json", false, "parse input as JSON (default)")
	fs.StringVar(&delimiterStr, "d", " ", "delimiter")

	fs.Parse(args)
	for fs.NArg() > 0 {
		fs.Parse(fs.Args()[1:])
	}

	p := new(params)
	if unjoro {
		if fromJSON {
			return nil, fmt.Errorf("can not specify `--from-json` with `-v`: deconverting only accepts Joro styled string.")
		}
		p.input = joroFormat
		p.output = jsonFormat

	} else {
		switch {
		case fromJSON && fromJoro:
			return nil, fmt.Errorf("can not specify `--from-json` with `--from-joro`")
		case fromJSON && !fromJoro, !fromJSON && !fromJoro:
			p.input = jsonFormat
		case !fromJSON && fromJoro:
			p.input = joroFormat
		}

		p.output = joroFormat

		switch {
		case padding && noPadding:
			return nil, fmt.Errorf("can not specify `--padding` with `--no-padding`.")
		case padding && !noPadding, !padding && !noPadding:
			p.hasPadding = true
		case !padding && noPadding:
			p.hasPadding = false
		}

		p.isTable = isTable

		if utf8.RuneCountInString(delimiterStr) != 1 {
			return nil, fmt.Errorf("the delimiter is not one character")
		}
		p.delimiter, _ = utf8.DecodeRuneInString(delimiterStr)
		if p.delimiter == utf8.RuneError {
			return nil, fmt.Errorf("invalid string")
		}
	}

	return p, nil
}

type strWithWidth struct {
	s string
	w int
}

func toStrWithWidthTable(rows [][]string) [][]strWithWidth {
	var widths []int
	for _, row := range rows {
		for i, col := range row {
			if len(widths) == i {
				widths = append(widths, 0)
			}

			widths[i] = maxInt(widths[i], len(col))
		}
	}

	if len(widths) == 0 {
		return [][]strWithWidth{}
	}

	var ret [][]strWithWidth
	for _, row := range rows {
		var strs []strWithWidth
		for i, col := range row {
			strs = append(strs, strWithWidth{
				s: col,
				w: widths[i],
			})
		}

		ret = append(ret, strs)
	}

	return ret
}

func toStrWithWidthNotTable(rows [][]string) [][]strWithWidth {
	var ret [][]strWithWidth

	var width int
	for _, row := range rows {
		if len(row) == 0 {
			for range rows {
				ret = append(ret, []strWithWidth{})
			}
			return ret
		}
		width = maxInt(width, len(row[0]))
	}

	for i := 0; i < len(rows); {
		colv := rows[i][0]
		var subRows [][]string
		for ; i < len(rows); i++ {
			if rows[i][0] != colv {
				break
			}

			subRows = append(subRows, rows[i][1:])
		}

		subrets := toStrWithWidthNotTable(subRows)
		for _, subret := range subrets {
			ret = append(ret, append([]strWithWidth{{s: colv, w: width}}, subret...))
		}
	}
	return ret
}

type table struct {
	columnNames []string
	data        []map[string]string
}

func toTableRows(dat interface{}) [][]string {
	var ret [][]string

	var ml maplike
	switch dt := dat.(type) {
	case []interface{}:
		ml = sliceMaplike(dt)
	case map[string]interface{}:
		ml = mapMaplike(dt)
	default:
		panic("not maplike dat") // TODO: accept string?
	}

	table := tototot(ml)

	ret = append(ret, table.columnNames)

	for _, m := range table.data {
		var row []string
		for _, name := range table.columnNames {
			column, ok := m[name]
			if !ok {
				row = append(row, "")
				continue
			}

			row = append(row, column)
		}

		ret = append(ret, row)
	}

	return ret
}

func toRows(dat interface{}) [][]string {
	var ret [][]string

	switch d := dat.(type) {
	case nil:
		ret = [][]string{{"<null>"}}
	case string:
		ret = [][]string{{escape(d)}}
	case bool:
		ret = [][]string{{fmt.Sprintf("<%t>", d)}}
	case float64:
		ret = [][]string{{fmt.Sprintf("<%v>", d)}}
	case []interface{}:
		for i, item := range d {
			pret := prefixSl(fmt.Sprintf("#%d", i), toRows(item))
			ret = append(ret, pret...)
		}

	case map[string]interface{}:
		for _, key := range keys(d) {
			pret := prefixSl(escape(key), toRows(d[key]))
			ret = append(ret, pret...)
		}
	}

	return ret
}

func prefixSl(p string, ss [][]string) [][]string {
	var ret [][]string
	for _, s := range ss {
		var cols []string
		cols = append(cols, p)
		cols = append(cols, s...)
		ret = append(ret, cols)
	}

	return ret
}

func tototot(m maplike) *table {
	nameSet := make(map[string]struct{})
	var hasValue bool
	var data []map[string]string
	for _, key := range m.keys() {
		values := make(map[string]string)

		switch keyT := key.(type) {
		case string:
			values["<Index>"] = keyT
		case int:
			values["<Index>"] = fmt.Sprintf("#%d", keyT)
		}

		switch v := m.at(key).(type) {
		case float64, nil, bool:
			values["<Value>"] = fmt.Sprintf("<%v>", v)
			hasValue = true
		case string:
			values["<Value>"] = v
			hasValue = true
		case map[string]interface{}:
			for k, vv := range v {
				nameSet[k] = struct{}{}
				switch vvt := vv.(type) {
				case float64, nil, bool:
					values[k] = fmt.Sprintf("<%v>", vvt)
				case string:
					values[k] = escape(vvt)
				case map[string]interface{}:
					values[k] = "<Object>"
				case []interface{}:
					values[k] = fmt.Sprintf("<Array(%d)>", len(vvt))
				default:
					panic(v)
				}
			}
		case []interface{}:
			for i, vv := range v {
				k := fmt.Sprintf("#%d", i)
				nameSet[k] = struct{}{}
				switch vvt := vv.(type) {
				case float64, nil, bool:
					values[k] = fmt.Sprintf("<%v>", vvt)
				case string:
					values[k] = escape(vvt)
				case map[string]interface{}:
					values[k] = "<Object>"
				case []interface{}:
					values[k] = fmt.Sprintf("<Array(%d)>", len(vvt))
				default:
					panic(v)
				}
			}
		default:
			panic(fmt.Sprintln("invalid type: ", m.at(key)))
		}
		data = append(data, values)
	}

	var names []string
	for k := range nameSet {
		names = append(names, escape(k))
	}
	sort.Strings(names)
	names = append([]string{"<Index>"}, names...)
	if hasValue {
		names = append(names, "<Value>")
	}

	return &table{
		columnNames: names,
		data:        data,
	}
}

type maplike interface {
	at(interface{}) interface{}
	keys() []interface{}
}

type slMaplike struct {
	sl []interface{}
}

func sliceMaplike(sl []interface{}) maplike {
	return &slMaplike{
		sl: sl,
	}
}

func (m *slMaplike) at(k interface{}) interface{} {
	return m.sl[k.(int)]
}

func (m *slMaplike) keys() []interface{} {
	var ret []interface{}
	for i := range m.sl {
		ret = append(ret, i)
	}

	return ret
}

type mMaplike struct {
	m map[string]interface{}
}

func mapMaplike(m map[string]interface{}) maplike {
	return &mMaplike{
		m: m,
	}
}

func (m *mMaplike) at(k interface{}) interface{} {
	return m.m[k.(string)]
}

func (m *mMaplike) keys() []interface{} {
	var ret []interface{}
	for k := range m.m {
		ret = append(ret, k)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].(string) < ret[j].(string)
	})

	return ret
}

func keys(m map[string]interface{}) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}

	sort.Strings(ks)
	return ks
}

func escape(s string) string {
	if len(s) == 0 {
		return "<>"
	}

	s = strings.Replace(s, "\\", "\\\\", -1)
	s = strings.Replace(s, " ", "\\ ", -1)
	s = strings.Replace(s, "\n", "\\n", -1)
	if s[0] == '<' || s[0] == '#' {
		s = "\\" + s
	}

	return s
}

func putMargin(orig string, width int) string {
	return orig + createMargin(width-len(orig))
}

func createMargin(n int) string {
	c := ' '

	var rs []rune
	for i := 0; i < n; i++ {
		rs = append(rs, c)
	}

	return string(rs)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
