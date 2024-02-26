package model

import (
	"errors"
	"time"

	"github.com/asaskevich/govalidator"
)

type TransactionRepositoryInterface interface {
	Register(transaction *Transaction) error
	Save(transaction *Transaction) error
	Find(transactionId int8) (*Transaction, error)
}

type Transaction struct {
	id        int32     `gorm:"type:int;primary_key" valid:"required"`
	valor     int32     `valid:"required"`
	tipo      string    `valid:"required"`
	descricao string    `valid:"required"`
	createdAt time.Time `valid:"required"`
}

func (transaction *Transaction) isValid() error {
	_, err := govalidator.ValidateStruct(transaction)
	if err != nil {
		return err
	}

	err = transaction.isAmountValid()
	if err != nil {
		return err
	}

	err = transaction.isTypeValid()
	if err != nil {
		return err
	}

	err = transaction.isDescriptionValid()
	if err != nil {
		return err
	}

	return nil
}

func (transaction *Transaction) isAmountValid() error {
	if transaction.valor <= 0 {
		return errors.New("amount must be greater than 0 in Transaction")
	}
	return nil
}

func (transaction *Transaction) isTypeValid() error {
	if transaction.tipo != "c" || transaction.tipo != "d" {
		return errors.New("type must be 'c' for credit or 'd' for debit")
	}
	return nil
}

func (transaction *Transaction) isDescriptionValid() error {
	length := len(transaction.descricao)
	if length <= 0 && length > 10 {
		return errors.New("description shoul'd have between 1 to 10 chars")
	}
}

func NewTransaction(valor int32, tipo string, descricao string, id int) (*Transaction, error) {
	transaction := Transaction{
		id:        id,
		valor:     valor,
		tipo:      tipo,
		descricao: descricao,
		createdAt: time.Now(),
	}

	err := transaction.isValid()
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}
