package main

import (
	"fmt"
	"sync"
)

// 交叉打印奇偶数/数字字母
func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	ch := make(chan struct{})
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			ch <- struct{}{}
			if i%2 != 0 {
				fmt.Println("g1: ", i)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			<-ch
			if i%2 == 0 {
				fmt.Println("g2: ", i)
			}
		}
	}()
	wg.Wait()
}

func main01() {
	var wg sync.WaitGroup
	wg.Add(1)
	ch01 := make(chan struct{})
	ch02 := make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-ch01:
				fmt.Println("g1 :", i)
				i++
				ch02 <- struct{}{}
			}
		}

	}()
	go func() {
		i := 'A'
		for {
			select {
			case <-ch02:
				if i >= 'Z' {
					wg.Done()
					return
				}
				fmt.Println("g2 :", string(i))
				i++
				ch01 <- struct{}{}
			}
		}
	}()
	ch01 <- struct{}{}
	wg.Wait()
}
