package usecase

import (
	"errors"
	"rinha/domain/model"
)

type TransactionUseCase struct {
	TransactionRepository model.TransactionRepositoryInterface
	AccountRepository     model.AccountRepositoryInterface
}

func (t *TransactionUseCase) Register(id int32, valor int32, descricao string, tipo string) (*model.Transaction, error) {
	account, err := t.AccountRepository.Find(id)
	if err != nil {
		return nil, err
	}

	if tipo == "c" {
		account.removeLimite(valor)
	}

	transaction, err := model.NewTransaction(valor, tipo, descricao, id)

	if err != nil {
		return nil, err
	}

	t.TransactionRepository.Save(transaction)
	if transaction.id != "" {
		return transaction, nil
	}
	return nil, errors.New("unable to process this transaction")
}
