package canonjson

import "testing"

func TestMarshalSortsKeysAndDisablesHTMLEscaping(t *testing.T) {
	in := map[string]any{
		"b": "x>y",
		"a": []any{2, 1},
		"c": map[string]any{"z": 1, "m": 2},
	}
	got, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": [\n    2,\n    1\n  ],\n  \"b\": \"x>y\",\n  \"c\": {\n    \"m\": 2,\n    \"z\": 1\n  }\n}\n"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMarshalIsStableAcrossCalls(t *testing.T) {
	in := map[string]any{"k1": 1, "k2": 2, "k3": 3, "k4": 4}
	a, _ := Marshal(in)
	b, _ := Marshal(in)
	if string(a) != string(b) {
		t.Fatal("marshal not stable")
	}
}
