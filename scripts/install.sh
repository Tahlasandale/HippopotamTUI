#!/bin/sh
#
# Installation de HippopotamTUI depuis une version publiée.
#
#   curl -fsSL https://github.com/Tahlasandale/HippopotamTUI/releases/latest/download/install.sh | sh
#
# Le script télécharge l'archive correspondant à la plateforme, vérifie son
# empreinte, et dépose le binaire dans ~/.local/bin. Il n'a besoin de rien
# d'autre que d'un shell et de curl.

set -eu

projet="Tahlasandale/HippopotamTUI"
binaire="hippo"

# Un terminal coloré rend les erreurs lisibles ; on n'en fait pas un prérequis.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    gras=$(printf '\033[1m'); rouge=$(printf '\033[31m'); fin=$(printf '\033[0m')
else
    gras=""; rouge=""; fin=""
fi

echouer() {
    printf '%s%s : %s%s\n' "$rouge" "$binaire" "$1" "$fin" >&2
    exit 1
}

# ---------------------------------------------------------------- plateforme

detecter_os() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux|darwin|freebsd) ;;
        *) echouer "plateforme non gérée : $os" ;;
    esac
    printf '%s' "$os"
}

detecter_arch() {
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) printf 'amd64' ;;
        aarch64|arm64) printf 'arm64' ;;
        *) echouer "architecture non gérée : $arch" ;;
    esac
}

# --------------------------------------------------------------- installation

repertoire() {
    if [ -n "${HIPPO_INSTALL_DIR:-}" ]; then
        printf '%s' "$HIPPO_INSTALL_DIR"
    elif [ -d "$HOME/.local/bin" ]; then
        printf '%s' "$HOME/.local/bin"
    else
        printf '%s' "$HOME/.local/bin"
    fi
}

# Les utilitaires sont recherchés avant le téléchargement : inutile de tirer
# vingt mégaoctets pour découvrir qu'il n'y a pas de tar.
exiger() {
    command -v "$1" >/dev/null 2>&1 || echouer "$1 est nécessaire mais absent"
}

exiger curl
exiger tar

os=$(detecter_os)
arch=$(detecter_arch)
cible="${os}_${arch}"
dossier=$(repertoire)

printf '%s%s%s pour %s/%s\n' "$gras" "$binaire" "$fin" "$os" "$arch"

# Une archive temporaire, effacée quoi qu'il arrive.
archive=$(mktemp -t hippo.XXXXXX)
trap 'rm -f "$archive"' EXIT

url="https://github.com/$projet/releases/latest/download/hippo_${cible}.tar.gz"
printf 'téléchargement %s\n' "$url"
curl -fsSL "$url" -o "$archive" || echouer "téléchargement impossible"

printf 'vérification de l''empreinte\n'
if curl -fsSL "https://github.com/$projet/releases/latest/download/SHA256SUMS" -o "$archive.sums" 2>/dev/null; then
    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s  %s\n' "$(sha256sum "$archive" | cut -d' ' -f1)" "hippo_${cible}.tar.gz" \
            | (cd "$(dirname "$archive")" && sha256sum -c - >/dev/null 2>&1) \
            || echouer "l'archive ne correspond pas à son empreinte"
    elif command -v shasum >/dev/null 2>&1; then
        grep " hippo_${cible}.tar.gz$" "$archive.sums" > "$archive.a-verifier" 2>/dev/null || true
        if [ -s "$archive.a-verifier" ]; then
            attendu=$(cut -d' ' -f1 < "$archive.a-verifier")
            obtenu=$(shasum -a 256 "$archive" | cut -d' ' -f1)
            [ "$attendu" = "$obtenu" ] || echouer "l'archive ne correspond pas à son empreinte"
        fi
    fi
    rm -f "$archive.sums" "$archive.a-verifier"
else
    printf '%sempreinte non publiée, on continue%s\n' "$gras" "$fin"
fi

mkdir -p "$dossier"
tar -xzf "$archive" -C "$dossier" "$binaire" || echouer "extraction impossible"
chmod +x "$dossier/$binaire"

version_installée=$("$dossier/$binaire" version 2>/dev/null | cut -d' ' -f2 || printf '?')
printf '\n%s%s %s est installé dans %s%s\n' "$gras" "$binaire" "$version_installée" "$dossier" "$fin"

case ":$PATH:" in
    *":$dossier:"*) ;;
    *) printf '\nAjoutez cette ligne à votre configuration de shell :\n\n    export PATH="%s:$PATH"\n\n' "$dossier" ;;
esac

printf '\nEssayez :\n\n    hippo --demo\n\n'