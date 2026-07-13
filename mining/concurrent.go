package mining

import (
	"strings"
	"sync"
	"time"

	"toy-blockchain/block"
	"toy-blockchain/utils"
)

func ConcurrentMineBlock(
	b *block.Block,
	difficulty int,
	workers int,
) (int, time.Duration) {

	start := time.Now()
	target := strings.Repeat("0", difficulty)
	result := make(chan int)
	stop := make(chan bool)

	var once sync.Once

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			nonce := workerID

			for {
				select {
				case <-stop:
					return
				default:
				}

				testBlock := *b
				testBlock.Nonce = nonce

				hash := utils.CalculateHash(testBlock)

				if strings.HasPrefix(hash, target) {
					once.Do(func() {
						b.Nonce = nonce
						b.Hash = hash

						result <- nonce
						close(stop)
					})

					return
				}

				nonce += workers
			}
		}(i)
	}

	foundNonce := <-result

	return foundNonce, time.Since(start)
}