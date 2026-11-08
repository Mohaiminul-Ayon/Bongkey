package orders

import (
	"strings"
	"unicode"
)

// Red flags mark orders staff should look at twice before confirming. They never block an order:
// a real customer with a short address must still be able to buy, and staff confirm every order by phone.
const (
	FlagDisposableEmail = "disposable_email"
	FlagShortAddress    = "short_address"
	FlagOddName         = "odd_name"
	FlagRepeatPhone     = "repeat_phone"
	FlagPhoneOtherName  = "phone_other_name"
	FlagHighValueCOD    = "high_value_cod"
	FlagFastSubmit      = "fast_submit"
)

// highValueCOD is the cash-on-delivery total above which a refused parcel costs the shop a lot.
const highValueCOD = 10000

// repeatPhoneOrders is how many earlier orders from one phone on the same day make the next one a repeat.
const repeatPhoneOrders = 2

// disposableDomains are common throwaway mailbox services.
// shortcut: a small fixed list; replace it with a maintained list if throwaway emails become a real problem.
var disposableDomains = map[string]bool{
	"mailinator.com": true, "guerrillamail.com": true, "10minutemail.com": true, "tempmail.com": true,
	"temp-mail.org": true, "yopmail.com": true, "trashmail.com": true, "getnada.com": true,
	"sharklasers.com": true, "dispostable.com": true, "maildrop.cc": true, "throwawaymail.com": true,
}

// PhoneHistory is what earlier orders say about a checkout phone number.
type PhoneHistory struct {
	// OrdersToday counts earlier orders with this phone since the start of the day.
	OrdersToday int
	// OtherName is true when a recent order with this phone used a different name.
	OtherName bool
}

// riskFlags lists the reasons an order looks unusual; an empty list is the normal case.
func riskFlags(request CheckoutRequest, total int, method string, history PhoneHistory) []string {
	flags := []string{}
	if _, domain, ok := strings.Cut(request.CustomerEmail, "@"); ok && disposableDomains[domain] {
		flags = append(flags, FlagDisposableEmail)
	}
	if letters(request.ShippingAddress) < 10 || !strings.ContainsAny(strings.TrimSpace(request.ShippingAddress), " ,") {
		flags = append(flags, FlagShortAddress)
	}
	if letters(request.CustomerName) < 2 || strings.ContainsFunc(request.CustomerName, unicode.IsDigit) {
		flags = append(flags, FlagOddName)
	}
	if history.OrdersToday >= repeatPhoneOrders {
		flags = append(flags, FlagRepeatPhone)
	}
	if history.OtherName {
		flags = append(flags, FlagPhoneOtherName)
	}
	if method == PayCashOnDelivery && total >= highValueCOD {
		flags = append(flags, FlagHighValueCOD)
	}
	if request.FastSubmit {
		flags = append(flags, FlagFastSubmit)
	}
	return flags
}

// letters counts letters in any script, so Bangla names and addresses count like English ones; Bangla vowel
// signs are Unicode marks, not letters, so marks count too.
func letters(value string) int {
	count := 0
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsMark(r) {
			count++
		}
	}
	return count
}
