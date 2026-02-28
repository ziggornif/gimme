# Rapport de revue de sécurité — `refactor/project-refresh`

**Date** : 2026-02-25
**Branche** : `refactor/project-refresh`
**Fichiers analysés** : 51
**Outil** : Claude Code (analyse statique + agents de validation)

---

## Résultat global

**Aucune vulnérabilité exploitable n'a été identifiée.** Toutes les pistes investiguées se sont révélées être des faux positifs après analyse approfondie du code.

---

## Pistes investiguées (toutes écartées)

### 1. Path Traversal dans l'extraction ZIP

**Fichier** : `internal/content/content-service.go` — ligne ~106
**Verdict** : Faux positif

**Analyse** : La regex `^[a-zA-Z0-9-_]+` ne supprime que le préfixe alphanumérique d'un chemin ZIP, laissant potentiellement des séquences `../` dans la clé résultante (ex. `pkg@1.0.0/../../../etc/passwd`). En apparence, cela ressemble à un path traversal.

**Pourquoi ce n'est pas exploitable** : Minio et S3 traitent les clés d'objets comme des chaînes littérales opaques. Une clé contenant `../` est simplement stockée avec cet identifiant exact — aucun mécanisme de résolution de chemin n'est appliqué. Il n'est pas possible d'accéder à un autre objet du bucket via cette technique. Un path traversal réel nécessiterait un backend de stockage sur système de fichiers local.

**Action corrective** : Aucune action de sécurité requise. En revanche, la regex mériterait d'être clarifiée pour exprimer explicitement son intention (voir section "Recommandations de qualité").

---

### 2. JWT Algorithm Confusion Attack

**Fichier** : `internal/auth/auth-manager.go` — ligne ~72
**Verdict** : Faux positif

**Analyse** : Le callback de `jwt.Parse()` retourne la clé secrète sans vérifier explicitement l'algorithme utilisé dans le token. En théorie, un attaquant pourrait forger un token signé avec l'algorithme `"none"`.

**Pourquoi ce n'est pas exploitable** : La bibliothèque `golang-jwt/jwt` v4.5.2 rejette explicitement l'algorithme `"none"` au niveau du parsing — aucun `SigningMethodNone` n'est enregistré. `jwt.Parse()` retourne une erreur pour tout algorithme non reconnu avant même d'invoquer le callback. L'absence de vérification explicite de `token.Method` dans le callback est un manque de défense en profondeur, pas une vulnérabilité exploitable.

**Action corrective** : Aucune action de sécurité requise. L'ajout d'une vérification explicite de l'algorithme est recommandé comme hardening (voir section "Recommandations de qualité").

---

### 3. XSS via noms de fichiers dans les templates

**Fichiers** : `templates/package.tmpl`, `internal/content/content-service.go`
**Verdict** : Faux positif

**Analyse** : Les noms de fichiers issus des clés S3 sont rendus dans le template via `{{.Name}}` dans un attribut `href`. Si un fichier uploadé portait un nom contenant du HTML/JavaScript, il pourrait théoriquement être injecté.

**Pourquoi ce n'est pas exploitable** : Deux protections indépendantes bloquent ce vecteur :
1. La regex d'extraction ZIP (`^[a-zA-Z0-9-_]+`) filtre tous les caractères spéciaux à l'upload — les guillemets, chevrons et espaces ne peuvent pas atteindre S3.
2. Gin utilise le package `html/template` de Go (et non `text/template`), qui applique un échappement contextuel automatique dans les attributs HTML — `"` devient `&#34;`, rendant toute injection d'event handler impossible.

**Action corrective** : Aucune action requise.

---

## Points positifs identifiés

Ces éléments du PR améliorent activement la posture de sécurité du projet :

| # | Description | Fichier(s) |
|---|-------------|------------|
| ✅ | **CVE-2024-51744 corrigée** — `golang-jwt/jwt` mis à jour de v4.4.1 → v4.5.2 (mauvaise gestion d'erreur dans `ParseWithClaims`) | `go.mod` |
| ✅ | **Gestion d'erreurs auth durcie** — `POST /create-token` retourne désormais un `400 Bad Request` explicite au lieu de fuiter les erreurs internes du décodeur JSON (`"EOF"`, `json: cannot unmarshal...`) | `api/auth-controller.go` |
| ✅ | **Versions CDN épinglées** — `redoc@latest` et `@picocss/pico@latest` remplacés par des versions fixes, éliminant le risque supply-chain lié aux tags mutables | `templates/` |
| ✅ | **Conteneur non-root** — Le Dockerfile crée et utilise un utilisateur `gimme` non-root, réduisant le blast radius en cas d'évasion de conteneur | `Dockerfile` |
| ✅ | **Protection des credentials** — `gimme.yml` ajouté au `.gitignore`, empêchant les commits accidentels de credentials | `.gitignore` |
| ✅ | **Propagation du contexte HTTP** — Les appels S3 utilisent désormais le contexte HTTP au lieu de `context.Background()`, permettant l'annulation correcte des requêtes | `internal/storage/`, `internal/content/` |
| ✅ | **Erreurs goroutines propagées** — Remplacement de `sync.WaitGroup` par `errgroup` dans `CreatePackage` : les erreurs d'upload ne sont plus silencieuses | `internal/content/content-service.go` |

---

## Recommandations de qualité (non bloquantes)

Ces points n'ont pas d'impact de sécurité immédiat mais améliorent la robustesse et la lisibilité du code.

### R1 — Vérification explicite de l'algorithme JWT

**Fichier** : `internal/auth/auth-manager.go` (~ligne 72)
**Priorité** : Basse

Ajouter une vérification explicite de l'algorithme dans le callback de `jwt.Parse()` pour documenter l'intention et se prémunir contre de futurs changements de bibliothèque :

```go
decoded, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    return []byte(am.secret), nil
})
```

---

### R2 — Clarifier la regex de renommage de fichiers ZIP

**Fichier** : `internal/content/content-service.go` (~ligne 33 et 106)
**Priorité** : Basse

La regex `^[a-zA-Z0-9-_]+` avec `ReplaceAllString` remplace uniquement le premier segment alphanumérique du chemin ZIP par le nom du package. L'intention n'est pas évidente à la lecture. Envisager de remplacer ce mécanisme par une logique explicite qui :
1. Split le chemin ZIP sur `/`
2. Remplace le premier segment par `folderName`
3. Rejoint les segments restants

```go
// Approche plus lisible
parts := strings.SplitN(currentFile.FileHeader.Name, "/", 2)
var fileName string
if len(parts) == 2 {
    fileName = folderName + "/" + parts[1]
} else {
    fileName = folderName + "/" + parts[0]
}
```

---

## Conclusion

La branche `refactor/project-refresh` présente une posture de sécurité saine. Les corrections apportées (CVE jwt, gestion d'erreurs auth, CDN épinglé, conteneur non-root) constituent des améliorations concrètes. Les deux recommandations de qualité listées ci-dessus sont facultatives et ne doivent pas bloquer la fusion.
