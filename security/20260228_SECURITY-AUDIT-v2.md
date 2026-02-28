# Audit de sécurité v2 — PR #3 : Focus gestion de tokens

**Date** : 2026-02-28
**Branche** : `refactor/project-refresh`
**Périmètre** : Nouveau système de gestion de tokens (tokens opaques, stores Redis/mémoire, OIDC, admin UI)
**Outil** : Claude Opus 4.6 — analyse statique ciblée

---

## Résultat global

**1 finding actionnable (MEDIUM)** identifié. La posture de sécurité du système de tokens est solide dans son ensemble.

---

## Finding

### 1. Credentials Redis exposés dans les logs applicatifs — MEDIUM

**Fichier** : `internal/auth/redis-token-store.go` — lignes 70, 79, 82
**Confiance** : 9/10
**Catégorie** : Exposition de credentials

**Description** : `NewRedisTokenStore()` logue l'URL Redis complète à trois endroits. Les URLs Redis contiennent fréquemment des credentials embarqués au format `redis://user:password@host:port/db`. Le mot de passe est écrit en clair dans les logs au niveau INFO.

Ce comportement est **incohérent** avec le pattern sûr déjà utilisé dans `internal/cache/redis-cache.go:34` qui logue uniquement `opts.Addr` (host:port, sans credentials).

**Scénario d'exploitation** : Un opérateur configure `redis://default:s3cretPass@redis.internal:6379` comme URL du token store. Au démarrage, le mot de passe apparaît dans les logs applicatifs. Toute personne ayant accès aux logs (ELK, Datadog, dashboards de monitoring) peut extraire le mot de passe Redis et se connecter directement au token store pour lire, modifier ou supprimer les hash de tokens API.

**Correction recommandée** : Loguer uniquement `opt.Addr` au lieu de l'URL complète :

```go
// Ligne 70 : message d'erreur
return nil, fmt.Errorf("redis-token-store: invalid URL: %w", err)
// Ligne 79 : message d'erreur
return nil, fmt.Errorf("redis-token-store: cannot reach Redis at %q: %w", opt.Addr, err)
// Ligne 82 : log info
logrus.Infof("[RedisTokenStore] connected to Redis at %s", opt.Addr)
```

---

## Points positifs confirmés

| # | Description | Fichier(s) |
|---|-------------|------------|
| ✅ | **Tokens opaques SHA-256** — pas de risque d'algorithm confusion JWT pour les tokens API | `auth-manager.go` |
| ✅ | **Comparaison constant-time** via `subtle.ConstantTimeCompare` dans le lookup de tokens | `memory-token-store.go`, `redis-token-store.go` |
| ✅ | **Modèle single-exposure** — le token brut n'est retourné qu'à la création, seuls les hash sont stockés | `auth-manager.go`, `admin-controller.go` |
| ✅ | **Flux OIDC sécurisé** — state cookie (CSRF), validation du nonce (anti-replay), vérification d'algorithme (HS256), clé de signature dérivée par domaine | `oidc-provider.go` |
| ✅ | **Cookies de session** — HttpOnly, SameSite=Lax, flag Secure configurable avec avertissement en production | `oidc-provider.go` |
| ✅ | **Purge des tokens expirés** — ticker 5min pour le store mémoire, TTL Redis pour le store Redis | `memory-token-store.go`, `redis-token-store.go` |
| ✅ | **Shutdown graceful** — connexions cache et token store correctement fermées | `application.go` |
| ✅ | **Validation des entrées** — longueur du nom de token (max 255), validation semver, validation d'archive | `admin-controller.go`, `archive-validator.go` |

---

## Conclusion

Le nouveau système de gestion de tokens est bien conçu et implémente les bonnes pratiques de sécurité attendues. Le seul finding (MEDIUM) est un problème d'hygiène de logs avec un correctif simple, déjà démontré dans le module cache du même projet.
