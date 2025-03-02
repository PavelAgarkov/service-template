package data_struct

import (
	"fmt"
	"testing"
)

func Test_reverse_LL(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.PushBack(1)
	ll.PushBack(2)
	ll.PushBack(3)
	ll.PushBack(4)
	ll.PushBack(5)
	ll.PushBack(6)
	ll.PushBack(7)
	ll.PushBack(8)
	ll.PushBack(9)
	ll.PushBack(10)
	fmt.Println(ll.String())

	var prev *Node[int]
	current := ll.head
	ll.tail = ll.head

	for current != nil {
		next := current.next
		current.next = prev
		prev = current
		current = next
		ll.head = prev
		fmt.Println(ll.String())
	}

	fmt.Println(ll.String())

}
