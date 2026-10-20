<div align="center">

<img src="assets/logo.svg" width="88" alt="OpenCode Go">

# Plugin Provider OpenCode Go

**Intégrez en toute transparence les 37 modèles OpenCode Go à CLIProxyAPI via un plugin natif, propre et sans résidus**

[ 简体中文 ](README.md) | [ English ](README_en.md) | [ 日本語 ](README_ja.md) | [ Français ](README_fr.md) | [ 한국어 ](README_ko.md)

<p>
  <img src="https://img.shields.io/badge/CPA-v8.0.27+-blue?style=flat-square" alt="CPA Version">
  <img src="https://img.shields.io/badge/OpenCode_Go-37_Modèles-success?style=flat-square" alt="Modèles">
  <img src="https://img.shields.io/badge/Protocole-4_Familles-orange?style=flat-square" alt="Protocoles">
  <img src="https://img.shields.io/github/v/release/Pheobe-Southwood/cpa-opencodego?style=flat-square" alt="Release">
  <img src="https://img.shields.io/npm/v/cpa-opencodego?style=flat-square" alt="npm">
  <img src="https://img.shields.io/badge/Licence-MIT-lightgrey?style=flat-square" alt="Licence">
</p>

</div>

---

> **v2.0.2** : bascule les deux correctifs HTTP 400 (rôle developer et replay du reasoning_content) en mode par défaut `retry-only` (réparation paresseuse) avec persistance sticky par session ; ajoute le diagnostic de troncature de flux. Lisez le [guide de mise à niveau v2](docs/UPGRADE-v2.md) avant de mettre à jour.


## Points Forts et Valeur Clé

OpenCode Go réunit 37 modèles d'intelligence artificielle de premier plan (DeepSeek V4.1, Claude Haiku 5.5, Gemini 4 Argon, GPT 6 Luna, Grok 4.7, Qwen 3.8, Kimi K3, etc.). Cependant, ces modèles sont répartis sur 4 points de terminaison protocolaires hétérogènes et exigent des règles de routage réseau spécifiques.

Ce plugin apporte une intégration complète et immédiate à CLIProxyAPI (CPA) :
- **Couverture Complète des 4 Familles Protocolaires** : Unifie `chat/completions`, `responses`, `messages` et `generateContent`. Les applications clientes peuvent interroger n'importe quel modèle avec n'importe quel format protocolaire.
- **Routage Proxy Individuel par Modèle** : Configurez librement chaque modèle en mode **Proxy**, **Connexion directe** ou **Héritage global** pour respecter les restrictions régionales et maximiser les débits.
- **Console d'Administration Web Embarquée** : Pilotez vos clés d'API avec priorités pondérées, suivez les quotas en direct et analysez les métriques réelles de débit et de latence directement depuis le panneau latéral de CPA.
- **Intégration Propre et Zéro Résidu** : Fonctionne sous forme de bibliothèques dynamiques (`.so`) sans modifier le code source de CPA ni toucher aux bases de données. La désinstallation est totale et sans traces.

---

## Visite Guidée des Fonctionnalités

### 1. Choix du Proxy par Modèle et Alias Personnalisés

> **Point fort** : Définissez une politique de sortie réseau distincte pour chaque modèle et configurez des alias de renommage pour les clients.

![Choix du Proxy par Modèle et Alias](docs/screenshots/desktop-models.png)

- **Sélection du Proxy Dédiée par Modèle** :
  - `Par proxy (proxy)` : Achemine le trafic via un proxy (indispensable pour les modèles soumis à des restrictions géographiques comme GPT 6 Luna et Grok 4.7).
  - `Connexion directe (direct)` : Force une connexion directe sans passer par le proxy (obligatoire pour Gemini 4 Argon Contributor afin d'éviter les erreurs 403).
  - `Hériter (inherit)` : Applique automatiquement la configuration de proxy du plugin ou le proxy global défini dans CPA.
- **Renommage par Alias** : Définissez le nom exposé aux clients (par exemple, renommer `deepseek-v4.1-flash` en `DeepSeek-V4.1-Flash`). Les listes de modèles et le routage CPA adoptent immédiatement cet identifiant.
- **Déduction et Sondage Automatique du Protocole** : Les nouveaux modèles en amont sont détectés via leur préfixe, avec un bouton « Sonder le protocole » pour tester concrètement la compatibilité en conditions réelles.

---

### 2. Gestion Multi-Clés et Suivi des Quotas en Temps Réel

> **Point fort** : Visualisez l'état des quotas de l'amont sur 3 fenêtres distinctes (5 heures, semaine, mois) avec basculement automatique.

![Gestion Multi-Clés et Quotas en Temps Réel](docs/screenshots/desktop-accounts.png)

- **Visualisation Multi-Fenêtres des Quotas** : Interroge directement l'API officielle `/usage` d'OpenCode Go. Affiche les pourcentages restants et le compte à rebours de réinitialisation pour les fenêtres de 5 heures glissantes, hebdomadaire et mensuelle.
- **Ordonnancement Pondéré par Priorité** : Enregistrez plusieurs clés d'API avec des niveaux de priorité distincts. CPA gère la répartition de charge et le basculement en cas de panne de manière transparente.
- **Renommage Instantané sur Place** : Modifiez le nom d'une clé en un clic directement depuis l'interface ; la configuration se recharge à chaud sans coupure de service.

---

### 3. Analyse Complète de l'Utilisation et Performances Réelles

> **Point fort** : Suivi précis de la consommation de jetons, conversion tarifaire officielle, débit global (TPS), vitesse de génération pure et temps de premier jeton (TTFT).

![Analyse Complète de l'Utilisation et Performances](docs/screenshots/desktop-stats.png)

- **Mesures de Performance Réelles** :
  - **TPS Global** : Jetons de sortie valides divisés par la durée totale de la requête côté hôte (reflète la vitesse globale de bout en bout).
  - **TPS de Génération** : Mesuré entre le premier et le dernier fragment de contenu, reflétant la vitesse pure de décodage du modèle.
  - **TTFT Moyen** : Temps écoulé entre l'envoi de la requête à l'amont et la réception du premier jeton effectif.
- **Suivi Budgétaire et Quotas** : Conversion automatique basée sur la grille tarifaire officielle pour les forfaits Go et Go Plus, affichant la consommation par clé pour chaque modèle.
- **Rappels de Mise à Jour** : Une alerte discrète s'affiche tous les 30 jours ou lors de modifications du catalogue amont pour vérifier la conformité des barèmes.
- **Journal des Requêtes** : Un nouvel onglet liste chaque tentative amont avec l'heure, les jetons d'entrée/sortie/total, le TTFT, le TPS de génération, le TPS de bout en bout, le statut et l'erreur. Les échecs et le `x-opencode-log-id` amont y sont consultables.

---

### 4. Architecture Intégrée des 4 Familles Protocolaires

> **Point fort** : Une configuration unique pilote simultanément les 4 protocoles, libérant totalement vos applications clientes des incompatibilités.

![Architecture Intégrée des 4 Familles Protocolaires](docs/screenshots/desktop-status.png)

- **Quatre Composants, Un Seul Produit** : Le composant principal fournit l'interface Web et la configuration, tandis que les trois sous-composants (Responses, Messages, Gemini) partagent automatiquement les paramètres en mémoire.
- **Traduction Trans-Protocolaire Transparente** : Un client formaté pour OpenAI peut interroger un modèle Claude ; un client formaté pour Anthropic peut solliciter Gemini 4 ou DeepSeek sans adaptation.
- **Configuration Zéro Tracas** : Seul le plugin principal nécessite des paramètres, les modules auxiliaires fonctionnent sans configuration supplémentaire.

---

## Démarrage Rapide

### Étape 1 : Déploiement des Fichiers du Plugin

Copiez les 4 fichiers `.so` compilés dans le dossier des plugins de CPA (généralement `plugins/linux/amd64/`) :
- `cpa-opencodego-v2.0.2.so`
- `cpa-opencodego-responses-v2.0.2.so`
- `cpa-opencodego-messages-v2.0.2.so`
- `cpa-opencodego-gemini-v2.0.2.so`

### Étape 2 : Ajout de la Configuration

Ajoutez le bloc suivant sous la section `plugins:` du fichier `config.yaml` de CPA :

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      developer-role: "retry-only"
      reasoning-replay: "retry-only"
      request-log: true
      keys:
        - name: "primary-key"
          key: "oc_sk_your_api_key_here"
          priority: 10
          enabled: true
    cpa-opencodego-responses: { enabled: true }
    cpa-opencodego-messages:  { enabled: true }
    cpa-opencodego-gemini:    { enabled: true }
```

### Étape 3 : Utilisation Immédiate dans l'Interface

1. Démarrez ou redémarrez votre instance CLIProxyAPI.
2. Rendez-vous dans le panneau d'administration de CPA et cliquez sur **OpenCode Go** dans le menu latéral « Plugins ».
3. Activez vos modèles préférés, ajustez les stratégies de proxy individuelles et observez les performances en temps réel !

---

## Documentation Technique et Architecture

Pour examiner les mécanismes C ABI, les détails d'interception de flux SSE, la référence intégrale des paramètres de configuration et les procédures de compilation, consultez :

- 📖 [Spécifications Techniques et Référence de Configuration (docs/TECHNICAL.md)](docs/TECHNICAL.md)
- 📐 [Document de Conception Détaillé (docs/design.md)](docs/design.md)
- 🛡️ [Guide de Désinstallation et Rollback Sans Résidu (docs/rollback.md)](docs/rollback.md)
- 🧪 [Rapports de Tests de Recette (docs/acceptance.md)](docs/acceptance.md)

---

## Licence

Ce projet est distribué sous la [Licence MIT](LICENSE).
