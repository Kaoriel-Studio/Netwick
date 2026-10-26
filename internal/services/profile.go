package services

import (
	"github.com/kaoriel-studio/netwick/internal/profile"
)

type ProfileService struct{ st *State }

func NewProfileService(st *State) *ProfileService { return &ProfileService{st: st} }

func (p *ProfileService) Get() profile.View { return p.st.Prof.View() }

func (p *ProfileService) Seen() { p.st.Prof.MarkSeen() }

func (p *ProfileService) Reset() { p.st.Prof.Reset() }
