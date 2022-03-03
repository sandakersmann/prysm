package htr

/*
#include <hashtree.h>
*/
import "C"
import (
	"unsafe"
)

// VectorizedSha256 takes a list of roots and hashes them using CPU
// specific vector instructions. Depending on host machine's specific
// hardware configuration, using this routine can lead to a significant
// performance improvement compared to the default method of hashing
// lists.
func VectorizedSha256(inputList [][32]byte, outputList [][32]byte) {
	sPtr := unsafe.Pointer(&inputList[0])
	C.sha256_8_avx2((*C.uchar)(unsafe.Pointer(&outputList[0])), (*C.uchar)(sPtr), C.ulong(len(inputList)/2))
}
