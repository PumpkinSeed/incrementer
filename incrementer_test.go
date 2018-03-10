package incrmntr

import "testing"

func TestAdd(t *testing.T) {
	//i := New("couchbase://cb1,cb2", "increment", "", 999, 1)
	i := New("couchbase://localhost", "increment", "", 999, 1)
	i.Add("test")
	i.Add("test")
	i.Add("test")
}

func BenchmarkAdd(b *testing.B) {

	//inc := New("couchbase://cb1,cb2", "increment", "", 999, 1)
	inc := New("couchbase://localhost", "increment", "", 999, 1)
	for i := 0; i < b.N; i++ {
		err := inc.Add("test")
		if err != nil {
			b.Error(err)
		}
	}
}
