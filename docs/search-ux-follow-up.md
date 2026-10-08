# Recherche adaptée à l'exploitation — retour utilisateur du 8 octobre 2026

Demandes acceptées pendant la revue164, PR37. Les corrections de présentation
restent dans164 ; les comportements indépendants suivants demandent des petits
lots après la revue. Ce document décrit le travail restant, pas des fonctionnalités
déjà disponibles. L'estimation antérieure13–28 doit être réévaluée avec ce périmètre.

## Correction de présentation dans164

La liste affiche une date lisible jour/mois/année et heure, toujours UTC, avec
la date canonique conservée dans datetime. Seuls les comptes de livraison non
nuls sont affichés, avec des mots d'exploitation. Source, origine d'import,
offsets, génération, qualité temporelle et réserves restent accessibles dans
« Informations techniques », volet details/summary fermé et utilisable au clavier.
L'avertissement global sur la couverture et la remise finale reste visible.
Les faits, le détail, l'API, la pagination et la reconstruction ne changent pas.
Les chaînes étranges des fixtures sont synthétiques, certaines volontairement
hostiles pour vérifier l'échappement ; elles ne représentent pas des mails réels.

## Ordre des comportements suivants

1. **Dates faciles à saisir.** Calendrier et heure, fuseau explicite, choix
   dernières24h / aujourd'hui / hier /7jours / période personnalisée. UTC initial
   explicite ; une autre zone doit être choisie et convertie sans interprétation
   implicite. Conserver début inclusif/fin exclusive, plafond31jours et dates
   figées lors de la pagination. Tester minuit et changements d'heure pour les
   zones ajoutées. La correction164 ne simplifie que l'affichage des résultats.
2. **Opérateurs textuels.** Égal à, commence par, contient, termine par, d'abord
   sur les adresses disponibles. Valeurs littérales : % et _ ne deviennent pas
   des jokers SQL ; paramètres liés, longueurs et délais bornés. Définir
   explicitement la comparaison de casse, sans changer silencieusement les
   correspondances exactes actuelles. Mesurer le coût des recherches contient.
3. **Critères combinés.** Ajouter plusieurs critères et choisir tous (ET) ou au
   moins un (OU). Un expéditeur et un destinataire peuvent être observés sur des
   événements différents : leur combinaison porte sur le même candidat/génération
   reconstruit dans l'instance sélectionnée, pas un AND sur une seule ligne.
   Ne pas mélanger des générations réutilisant un Queue ID. Définir aussi le
   traitement des événements NOQUEUE, absences et adresses explicitement vides.
   Adapter contrat, curseur, stockage/API puis formulaire par lots distincts.
4. **Sujet du mail (Subject).** Le champ n'est pas projeté ni indexé actuellement.
   Les journaux déjà importés ne permettent pas de l'inventer. Définir une source
   explicite, facultative, capable de fournir le sujet avec identité de file ;
   parser et stocker ses observations bornées avant d'ajouter le filtre.
   Postfix propose des inspections d'en-tête et une action WARN qui journalise
   l'observation : [documentation officielle](https://www.postfix.org/header_checks.5.html).
   Cette possibilité ne constitue pas encore une configuration de collecte
   validée pour QueueAtlas. Couvrir sujets repliés/encodés MIME, valeurs absentes,
   identités contradictoires et rétention ; aucune lecture du corps requise.

La demande Subject actualise le cadrage initial qui excluait le stockage du sujet
par défaut : collecte facultative explicite, absence affichée comme indisponible,
jamais assimilée à un sujet vide. AD/OIDC/Keycloak après MVP et MIT inchangés.

## Validation restante

Rapport humain acquis : login puis recherche ABC123 sur b845d95. Capture fournie
pour le tableau ; nouveau rendu164 non encore confirmé dans le navigateur.
Détail, trois événements de timeline et logout/accès refusé restent à vérifier.
Les attributs et la rotation des cookies/SameSite demandent leurs preuves propres.
L'outil navigateur ayant refusé l'accès, aucun contournement ni succès implicite.
