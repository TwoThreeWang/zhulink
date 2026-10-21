package handlers

import "testing"

func TestSearchTotalPages(t *testing.T) {
	cases := map[int64]int{0: 1, 1: 1, 30: 1, 31: 2, 60: 2, 61: 3}
	for total, want := range cases {
		if got := searchTotalPages(total); got != want {
			t.Errorf("searchTotalPages(%d) = %d, want %d", total, got, want)
		}
	}
}

func TestLikeEscaper(t *testing.T) {
	cases := map[string]string{"100%": `100\%`, "snake_case": `snake\_case`, `C:\dir`: `C:\\dir`, "普通": "普通"}
	for in, want := range cases {
		if got := likeEscaper.Replace(in); got != want {
			t.Errorf("likeEscaper(%q) = %q, want %q", in, got, want)
		}
	}
}
