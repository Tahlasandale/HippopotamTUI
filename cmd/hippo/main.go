// Command hippo est un client de messagerie en ligne de commande : un TUI léger
// et documenté en français au-dessus de himalaya.
//
// Le TUI ne lit jamais la configuration de himalaya, qui contient des mots de
// passe en clair. Il ne parle qu'à la ligne de commande.
package main

import (
	"fmt"
	"os"

	"github.com/Tahlasandale/HippopotamTUI/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "hippo :", err)
		os.Exit(1)
	}
}