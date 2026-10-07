# ADR-006 — Authentification du MVP

Statut : accepté le 3 octobre 2026 pour le MVP ; extensions d'identité planifiées après le MVP.

## Contexte
L'API et l'interface exposent adresses, IP, identifiants et logs potentiellement sensibles.
## Options
Compte local ; authentification au proxy ; OIDC.
## Décision
Écoute loopback par défaut et compte administrateur local initialisé en CLI ; toute route de données protégée. Bind distant seulement avec authentification et TLS direct ou proxy TLS.
## Raisons
Autonomie du binaire sans IAM externe obligatoire.
## Conséquences
Hash de mot de passe éprouvé, sessions révocables, cookies sûrs, CSRF, limitation des essais et tests de bypass. Mode proxy et OIDC futurs demandent des contrôles distincts.
## Limites
Gestion d'un mot de passe local. L'authentification par compte Active Directory et fournisseur OpenID Connect tel que Keycloak est demandée pour une future release ; voir ADR-008.

## État de mise en œuvre au lot 153

Compte/CLI143–147 et composants sessions/login/HTTP/garde/corpus148–152 réalisés.
Le chemin HTTP livré en bibliothèque exige TLS **direct**, même en loopback ;
TLS terminé au proxy puis HTTP vers QueueAtlas n'est pas pris en charge.
Les headers Forwarded ne donnent ni transport sûr ni identité. La variante proxy
de la décision initiale demande une conception distincte avant d'être proposée.
Montage applicatif, configuration TLS/listener et interface Web restent à réaliser.
La [revue du chantier](../reviews/m4-local-http.md) consigne limites et preuves.
