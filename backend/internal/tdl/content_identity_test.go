package tdl

import (
	"encoding/json"
	"testing"
)

func TestExportKeepsExactContentIdentityWithoutThumbnail(t *testing.T) {
	for _, fixture := range []struct{ raw, want string }{
		{`{"Media":{"Document":{"ID":9223372036854775000}}}`, "document:9223372036854775000"},
		{`{"media":{"photo":{"id":8223372036854775001}}}`, "photo:8223372036854775001"},
		{`{"Media":{"Document":{"ID":0}}}`, ""},
		{`{}`, ""},
	} {
		t.Run(fixture.want, func(t *testing.T) {
			items, err := ParseExport([]byte(`{"id":123,"messages":[{"id":7,"file":"a.pdf","raw":` + fixture.raw + `}]}`))
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(items[0])
			var item map[string]any
			json.Unmarshal(encoded, &item)
			got, _ := item["contentId"].(string)
			if got != fixture.want {
				t.Fatalf("identity=%q want=%q", got, fixture.want)
			}
		})
	}
}
