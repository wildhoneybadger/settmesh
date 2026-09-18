package auth

import (
	"sync"
	"time"
)

// LoginLimiter applique une limitation du nombre de tentatives de connexion par IP,
// avec un délai progressif (exponentiel) après des échecs répétés.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*ipAttempts

	// MaxFreeAttempts est le nombre d'échecs tolérés avant qu'un délai s'applique.
	MaxFreeAttempts int
	// BaseDelay est le délai appliqué au premier échec au-delà de MaxFreeAttempts.
	BaseDelay time.Duration
	// MaxDelay plafonne le délai progressif.
	MaxDelay time.Duration
}

type ipAttempts struct {
	failures     int
	blockedUntil time.Time
	lastSeen     time.Time
}

// NewLoginLimiter crée un limiteur avec des valeurs par défaut raisonnables :
// 5 essais libres, puis un délai doublant à chaque échec jusqu'à 5 minutes.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		attempts:        make(map[string]*ipAttempts),
		MaxFreeAttempts: 5,
		BaseDelay:       time.Second,
		MaxDelay:        5 * time.Minute,
	}
}

// Allow indique si une tentative de connexion depuis ip est autorisée maintenant, et
// si non, la durée à attendre avant de réessayer.
func (l *LoginLimiter) Allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[ip]
	if !ok {
		return true, 0
	}
	a.lastSeen = time.Now()

	if remaining := time.Until(a.blockedUntil); remaining > 0 {
		return false, remaining
	}
	return true, 0
}

// RecordFailure enregistre un échec de connexion depuis ip et met à jour le délai de
// blocage si le nombre d'échecs libres est dépassé.
func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[ip]
	if !ok {
		a = &ipAttempts{}
		l.attempts[ip] = a
	}
	a.failures++
	a.lastSeen = time.Now()

	if a.failures > l.MaxFreeAttempts {
		shift := a.failures - l.MaxFreeAttempts - 1
		if shift > 30 {
			shift = 30 // évite un débordement de time.Duration
		}
		delay := l.BaseDelay << uint(shift)
		if delay <= 0 || delay > l.MaxDelay {
			delay = l.MaxDelay
		}
		a.blockedUntil = time.Now().Add(delay)
	}
}

// RecordSuccess réinitialise le compteur d'échecs pour ip après une connexion réussie.
func (l *LoginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// Cleanup supprime les entrées inactives depuis plus de maxAge, pour éviter une
// croissance illimitée de la mémoire sur un serveur de longue durée.
func (l *LoginLimiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for ip, a := range l.attempts {
		if a.lastSeen.Before(cutoff) {
			delete(l.attempts, ip)
		}
	}
}
