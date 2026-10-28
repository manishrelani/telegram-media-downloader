package tdl

import (
	"encoding/json"
	"testing"
)

func TestThumbnailReferenceUsesExactIDsAndPrefersRetinaSizedImage(t *testing.T) {
	raw := []byte(`{"id":1,"messages":[{"id":1,"file":"a.mp4","raw":{"Media":{"Document":{"ID":9223372036854775000,"AccessHash":8223372036854775000,"FileReference":"AQI=","DCID":2,"Thumbs":[{"Type":"s","W":100,"H":56,"Size":3000},{"Type":"m","W":320,"H":180,"Size":24000},{"Type":"x","W":800,"H":450,"Size":100000}]}}}}]}`)
	items, err := ParseExport(raw)
	if err != nil {
		t.Fatal(err)
	}
	ref := items[0].Preview
	if ref == nil || ref.ID != 9223372036854775000 || ref.AccessHash != 8223372036854775000 || ref.ThumbSize != "m" {
		t.Fatalf("ref=%+v", ref)
	}
	encoded, _ := json.Marshal(items[0])
	var value map[string]any
	json.Unmarshal(encoded, &value)
	if _, ok := value["preview"]; ok {
		t.Fatal("private location leaked to UI")
	}
}
