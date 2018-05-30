package dbcontainer

import (
	"context"
	"strings"
	"testing"

	"github.com/aperturerobotics/objstore/db/inmem"
	// "github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/sets/treeset"
)

// TestTreeMap tests a tree-map db container.
// Currently failing.
/* https://github.com/emirpasic/gods/issues/71
func TestTreeMap(t *testing.T) {
	tm := treemap.NewWithIntComparator()
	tm.Put(1, "x")
	tm.Put(2, "b")
	tm.Put(1, "a")

	dbm := inmem.NewInmemDb()
	dbc := NewDbContainer(dbm, "test", tm)
	if err := dbc.WriteState(context.Background()); err != nil {
		t.Fatal(err.Error())
	}

	tmb := treemap.NewWithIntComparator()
	dbc = NewDbContainer(dbm, "test", tmb)
	if err := dbc.ReadState(context.Background()); err != nil {
		t.Fatal(err.Error())
	}

	vals := tmb.Values()
	if vals[0].(string) != "a" {
		t.Fail()
	}
	if vals[1].(string) != "b" {
		t.Fail()
	}
}
*/

func TestTreeSet(t *testing.T) {
	ts1 := treeset.NewWithStringComparator()
	ts1.Add("test-1", "test-2", "test-3")

	dbm := inmem.NewInmemDb()
	dbc := NewDbContainer(dbm, "test", ts1)
	if err := dbc.WriteState(context.Background()); err != nil {
		t.Fatal(err.Error())
	}

	ts2 := treeset.NewWithStringComparator()
	dbc = NewDbContainer(dbm, "test", ts2)
	if err := dbc.ReadState(context.Background()); err != nil {
		t.Fatal(err.Error())
	}

	vals1 := ts1.Values()
	vals2 := ts2.Values()

	for i, val := range vals1 {
		val1 := val.(string)
		val2 := vals2[i].(string)

		if strings.Compare(val1, val2) != 0 {
			t.Fail()
		}
	}
}
