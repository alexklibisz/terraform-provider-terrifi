package unifi

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Number is a JSON value that may arrive as a bare number or as a quoted
// string. Many UniFi v2 endpoints emit numeric fields as strings ("443",
// "3600"), so resource types that decode controller responses use this for
// the wire-level field and copy the typed value out in their UnmarshalJSON.
type Number json.Number

func (n *Number) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	if s == `""` {
		*n = ""
		return nil
	}
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		unq, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		*n = Number(unq)
		return nil
	}
	*n = Number(s)
	return nil
}

func (n Number) String() string            { return string(n) }
func (n Number) Float64() (float64, error) { return json.Number(n).Float64() }
func (n Number) Int64() (int64, error)     { return json.Number(n).Int64() }

func (n Number) Int64Pointer() *int64 {
	v, err := n.Int64()
	if err != nil {
		return nil
	}
	return &v
}
