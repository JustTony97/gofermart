package handler

import "net/http"

type BalanceHandler struct {
}

func (b *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
}
func (b *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
}
func (b *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
}
