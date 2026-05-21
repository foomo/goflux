package nats_test

import (
	"fmt"

	"github.com/foomo/goflux/subject/nats"
)

func ExampleNew() {
	s := nats.New()
	fmt.Printf("%q\n", s.String())
	// Output: ""
}

func ExampleNew_multipleSegments() {
	s := nats.New("prod", "acme")
	fmt.Println(s.String())
	// Output: prod.acme
}

func ExampleNewDomain() {
	d := nats.NewDomain("order")
	fmt.Println(d.String())
	// Output: order
}

func ExampleSubject_Domain() {
	d := nats.New("prod", "acme").Domain("user")
	fmt.Println(d.String())
	// Output: prod.acme.user
}

func ExampleDomain_String() {
	d := nats.New("prod", "acme").Domain("user")
	fmt.Println(d.String())
	// Output: prod.acme.user
}

func ExampleDomain_All() {
	d := nats.New("prod", "acme").Domain("user")
	fmt.Println(d.All())
	// Output: prod.acme.user.>
}

func ExampleDomain_Entity() {
	e := nats.NewDomain("user").Entity("profile")
	fmt.Println(e.String())
	// Output: user.profile
}

func ExampleEntity_String() {
	e := nats.NewDomain("user").Entity("profile")
	fmt.Println(e.String())
	// Output: user.profile
}

func ExampleEntity_All() {
	e := nats.NewDomain("order").Entity("item")
	fmt.Println(e.All())
	// Output: order.item.>
}

func ExampleEntity_Event() {
	ev := nats.NewDomain("order").Entity("item").Event("created")
	fmt.Println(ev.String())
	// Output: order.item.created
}

func ExampleEvent_String() {
	ev := nats.New("prod", "acme").
		Domain("user").
		Entity("profile").
		Event("updated")
	fmt.Println(ev.String())
	// Output: prod.acme.user.profile.updated
}
