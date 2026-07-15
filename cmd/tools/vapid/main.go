// Генератор пары VAPID-ключей для Web Push. Вывод готов для вставки в .env.
package main

import (
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "не удалось сгенерировать VAPID-ключи:", err)
		os.Exit(1)
	}
	fmt.Println("# добавьте эти строки в .env:")
	fmt.Println("VAPID_PUBLIC_KEY=" + pub)
	fmt.Println("VAPID_PRIVATE_KEY=" + priv)
}
