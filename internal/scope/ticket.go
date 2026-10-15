// Package scope demonstrates singleton and prototype scopes.
package scope

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"springlab/pkg/ioc"
)

// Listing 9. Ticket is @Scope("prototype"); TicketOffice is a singleton.
func init() {
	ioc.Component(NewTicket, ioc.ScopePrototype())
	ioc.Component(NewTicketOffice)
}

type Ticket struct{ id string }

func NewTicket() *Ticket {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return &Ticket{id: hex.EncodeToString(b)}
}

func (t *Ticket) ID() string { return t.id }

type TicketOffice struct {
	direct   *Ticket                // created ONCE
	provider *ioc.Provider[*Ticket] // source of new instances
}

func NewTicketOffice(direct *Ticket, provider *ioc.Provider[*Ticket]) *TicketOffice {
	return &TicketOffice{direct: direct, provider: provider}
}

func (o *TicketOffice) Demo() map[string]any {
	return map[string]any{
		"injectedDirectly": []string{o.direct.ID(), o.direct.ID()},
		"viaProvider":      []string{o.provider.GetObject().ID(), o.provider.GetObject().ID()},
		"office":           fmt.Sprintf("%p", o), // singleton identity
	}
}
