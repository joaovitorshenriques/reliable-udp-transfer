# Etapa 2 — Cliente e servidor

O servidor disponibiliza listagem, upload e download por UDP. Cada pedido usa autenticação por senha e um identificador que permite reconhecer repetições. A listagem é paginada e as transferências verificam a integridade por SHA-256.

## Executar

Crie `password.txt` com uma senha em uma linha. Na raiz do projeto, inicie o servidor:

```text
go run ./cmd/rc2 serve -addr 127.0.0.1:9000 -dir data/server -password-file password.txt
```

Em outro terminal, execute:

```text
go run ./cmd/rc2 upload -addr 127.0.0.1:9000 -file README.md -name exemplo.md -password-file password.txt
go run ./cmd/rc2 list -addr 127.0.0.1:9000 -password-file password.txt
go run ./cmd/rc2 download -addr 127.0.0.1:9000 -name exemplo.md -file recebido.md -password-file password.txt
```

A listagem inclui `exemplo.md`. Upload e download informam o resultado e o SHA-256. Use nomes novos para repetir a demonstração, pois arquivos existentes são preservados.

## Validação e limites

A suíte verifica listagem, envio, download e rejeição de senha incorreta, nomes inválidos e destinos existentes.

```text
go run ./cmd/validate
```

Use a autenticação em rede isolada, pois a senha trafega em texto claro. Os nomes remotos são simples, sem subpastas. Veja o [formato das mensagens](PROTOCOLO.md) e os [limites de concorrência](ETAPA3.md#atendimento-simultâneo-e-limites).
