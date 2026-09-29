package common

import (
	"errors"
	"slices"
	"testing"
)

func TestPaginate(t *testing.T) {
	sevenItems := []int{1, 2, 3, 4, 5, 6, 7}

	cases := []struct {
		name    string
		items   []int
		page    int
		size    int
		want    Page[int]
		wantErr string
		// wantNotFound expects a *PageNotFoundError.
		wantNotFound bool
	}{
		{
			name:  "the first page",
			items: sevenItems,
			page:  1,
			size:  3,
			want:  Page[int]{Items: []int{1, 2, 3}, Number: 1, TotalPages: 3, TotalItems: 7},
		},
		{
			name:  "a middle page",
			items: sevenItems,
			page:  2,
			size:  3,
			want:  Page[int]{Items: []int{4, 5, 6}, Number: 2, TotalPages: 3, TotalItems: 7},
		},
		{
			name:  "the last page holds what is left",
			items: sevenItems,
			page:  3,
			size:  3,
			want:  Page[int]{Items: []int{7}, Number: 3, TotalPages: 3, TotalItems: 7},
		},
		{
			name:  "a size of 0 puts every item on one page",
			items: sevenItems,
			page:  1,
			size:  0,
			want:  Page[int]{Items: sevenItems, Number: 1, TotalPages: 1, TotalItems: 7},
		},
		{
			name:    "a page below 1",
			items:   sevenItems,
			page:    0,
			size:    3,
			wantErr: "page must be 1 or more, got 0",
		},
		{
			name:    "a negative size",
			items:   sevenItems,
			page:    1,
			size:    -1,
			wantErr: "page size must be 0 or more, got -1",
		},
		{
			name:         "a page past the end",
			items:        sevenItems,
			page:         5,
			size:         3,
			wantErr:      "page 5 does not exist, there are 3 pages",
			wantNotFound: true,
		},
		{
			name:  "an empty list on page 1",
			items: nil,
			page:  1,
			size:  3,
			want:  Page[int]{Items: nil, Number: 1, TotalPages: 1, TotalItems: 0},
		},
		{
			name:         "an empty list on page 2",
			items:        nil,
			page:         2,
			size:         3,
			wantErr:      "page 2 does not exist, there is 1 page",
			wantNotFound: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Paginate(testCase.items, testCase.page, testCase.size)

			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected the error %q, got the page %+v", testCase.wantErr, got)
				}
				if err.Error() != testCase.wantErr {
					t.Errorf("error = %q, want %q", err, testCase.wantErr)
				}
				var notFound *PageNotFoundError
				if isNotFound := errors.As(err, &notFound); isNotFound != testCase.wantNotFound {
					t.Errorf("errors.As(err, *PageNotFoundError) = %t, want %t", isNotFound, testCase.wantNotFound)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got.Items, testCase.want.Items) {
				t.Errorf("Items = %v, want %v", got.Items, testCase.want.Items)
			}
			if got.Number != testCase.want.Number || got.TotalPages != testCase.want.TotalPages || got.TotalItems != testCase.want.TotalItems {
				t.Errorf("page %d of %d with %d items, want page %d of %d with %d items",
					got.Number, got.TotalPages, got.TotalItems, testCase.want.Number, testCase.want.TotalPages, testCase.want.TotalItems)
			}
		})
	}
}

func TestPageNotFoundError_NamesTheNoun(t *testing.T) {
	err := &PageNotFoundError{Number: 5, TotalPages: 3, Noun: "projects"}

	if want := "page 5 does not exist, there are 3 pages of projects"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
}
