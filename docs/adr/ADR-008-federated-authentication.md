# ADR-008 — Authentification Active Directory et OpenID Connect

Statut : besoin accepté le 3 octobre 2026 ; conception détaillée reportée à une release après le MVP.

## Contexte
QueueAtlas devra permettre à un administrateur de se connecter avec un compte Active Directory ou un compte géré par un fournisseur externe tel que Keycloak. Le MVP conserve son compte administrateur local autonome.

## Options
Pour Active Directory sur site : LDAP sur TLS (LDAPS ou StartTLS avec validation de certificat), ou fédération par un fournisseur OIDC relié à AD. Pour les fournisseurs externes : protocole OpenID Connect générique, plutôt qu'un adaptateur propre à Keycloak. Une authentification par proxy reste une autre option de déploiement à encadrer séparément.

## Décision proposée
Prévoir un modèle d'identité interne `(provider_id, subject_id)` et des rôles QueueAtlas explicites. Ajouter après le MVP un client OIDC standard testé avec Keycloak et un mode AD sécurisé, direct ou fédéré selon le besoin d'exploitation. Autoriser plusieurs fournisseurs configurés sans confondre des utilisateurs qui partagent la même adresse email. Conserver un compte local de secours sous contrôle de l'administrateur.

## Raisons
Keycloak expose les points d'entrée OIDC via la découverte standard ; Active Directory peut être interrogé par LDAP protégé par TLS. Une couche d'identité unique évite de lier les permissions applicatives au protocole de connexion. [Keycloak OIDC](https://www.keycloak.org/securing-apps/oidc-layers), [Microsoft LDAPS](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/enable-ldap-over-ssl-3rd-certification-authority)

## Conséquences
OIDC : flux Authorization Code avec PKCE, validation de `state`, `nonce`, issuer, audience, signature et rotation JWKS ; URL de découverte et redirections fixées par configuration administrateur. AD direct : TLS obligatoire avec certificat validé, identité de compte stable et mappage de groupes vers `viewer`/`admin` avec refus par défaut. Ne jamais faire d'une adresse email ou d'un nom d'affichage une clé d'autorisation. Tester déconnexion, expiration, indisponibilité de fournisseur, changement de groupes et collision de comptes.

## Limites
Le propriétaire devra préciser avant l'implémentation s'il s'agit d'AD DS sur site, d'Entra ID ou d'un AD fédéré, ainsi que les groupes à mapper. Le mode AD direct ajoute un chemin réseau et des secrets de service éventuels ; il n'est pas requis pour le MVP autonome.
