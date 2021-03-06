package aba

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

// Writer implements buffering for an io.Writer object.
// If an error occurs writing to a Writer, no more data will be
// accepted and all subsequent writes, and Flush, will return the error.
// After all data has been written, the client should call the
// Flush method to guarantee all data has been forwarded to
// the underlying io.Writer.
type Writer struct {
	// OmitBatchTotals can be used for banks that don't summarise
	// the credit/debit transactions
	OmitBatchTotals bool
	// CRLFLineEndings allows you to toggle whether to use Windows/DOS style
	// line endings vs the default unix style
	CRLFLineEndings bool
	header
	trailer
	wr *bufio.Writer
}

// NewWriter returns a new Writer whose buffer has the default size.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		wr: bufio.NewWriter(w),
		header: header{
			recordType:         0,
			fileSequenceNumber: 1,
			APCAUserID:         181,
			Description:        "Creditors",
			ProcessingDate:     time.Now(),
		},
		trailer: trailer{
			recordType: 7,
			DefaultBSB: "999-999",
		},
	}
}

// Write writes the provided []Records into the buffer.
// It returns an error if something is wrong with the records.
func (w *Writer) Write(records []Record) (err error) {
	if len(records) < 2 {
		return ErrInsufficientRecords
	}
	if len(strings.TrimSpace(w.NameOfUsersBank)) == 0 {
		return ErrMustSpecifyUsersBank
	}

	if len(strings.TrimSpace(w.NameOfUserID)) == 0 {
		return ErrMustSpecifyUsersID
	}
	if w.APCAUserID == 0 {
		return ErrMustSpecifyAPCAUserID
	}

	// Validation spin...
	w.trailer.userTotalRecords = len(records) // Count valid records
	for i, r := range records {
		if !r.IsValid() {
			return fmt.Errorf("%v (record %d)", ErrInvalidRecord, i)
		}
	}

	w.trailer.userCreditTotalAmount = 0
	w.trailer.userDebitTotalAmount = 0
	w.header.Write(w.wr)
	if w.CRLFLineEndings {
		w.wr.WriteByte('\r')
	}
	w.wr.WriteByte('\n')
	for i, r := range records {
		r.Write(w.wr)
		if !w.OmitBatchTotals {
			switch r.TransactionCode {
			case Debit:
				w.trailer.userDebitTotalAmount += r.Amount
			default:
				if strings.HasPrefix(r.TransactionCode, "5") {
					w.trailer.userCreditTotalAmount += r.Amount
				} else {
					log.Println("Unknown transaction type", r.TransactionCode, "in record", i)
				}
			}
		}
		if w.CRLFLineEndings {
			w.wr.WriteByte('\r')
		}
		w.wr.WriteByte('\n')
	}

	// Last part is to get net trailer amount
	// Some banks require a balancing line at the bottom
	// We're going to omit it unless told otherwise
	if w.trailer.userDebitTotalAmount < w.trailer.userCreditTotalAmount {
		w.trailer.userNetTotalAmount = w.trailer.userCreditTotalAmount - w.trailer.userDebitTotalAmount
	}
	w.trailer.Write(w.wr)
	return nil
}

// Flush can be called to ensure all data has been written
func (w *Writer) Flush() {
	w.wr.Flush()
}

// Error reports any error that has occurred during a previous Write or Flush.
func (w *Writer) Error() error {
	_, err := w.wr.Write(nil)
	return err
}
