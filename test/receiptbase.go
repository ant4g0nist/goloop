// Code generated by go generate; DO NOT EDIT.
package test

import (
	"math/big"

	"github.com/icon-project/goloop/module"
)

type ReceiptBase struct{}

func (_r *ReceiptBase) Bytes() []byte {
	panic("not implemented")
}

func (_r *ReceiptBase) To() module.Address {
	panic("not implemented")
}

func (_r *ReceiptBase) CumulativeStepUsed() *big.Int {
	panic("not implemented")
}

func (_r *ReceiptBase) StepPrice() *big.Int {
	panic("not implemented")
}

func (_r *ReceiptBase) StepUsed() *big.Int {
	panic("not implemented")
}

func (_r *ReceiptBase) Status() module.Status {
	panic("not implemented")
}

func (_r *ReceiptBase) SCOREAddress() module.Address {
	panic("not implemented")
}

func (_r *ReceiptBase) Check(r module.Receipt) error {
	panic("not implemented")
}

func (_r *ReceiptBase) ToJSON(version module.JSONVersion) (interface{}, error) {
	panic("not implemented")
}

func (_r *ReceiptBase) LogsBloom() module.LogsBloom {
	panic("not implemented")
}

func (_r *ReceiptBase) EventLogIterator() module.EventLogIterator {
	panic("not implemented")
}

func (_r *ReceiptBase) FeePaymentIterator() module.FeePaymentIterator {
	panic("not implemented")
}

func (_r *ReceiptBase) GetProofOfEvent(int) ([][]byte, error) {
	panic("not implemented")
}
