# RC2 — Transferência confiável sobre UDP

Projeto de Redes de Computadores II (CSI510), implementado em Go. O protocolo usa janela deslizante Selective Repeat, retransmissões seletivas e SHA-256 para verificar os arquivos. O projeto também inclui experimentos de desempenho, uma mini-CDN e um painel de métricas.

## Executar localmente

Use **Go 1.22 ou superior**. Os comandos abaixo funcionam em Windows e Linux e devem ser executados na raiz do repositório. Crie um arquivo UTF-8 chamado `password.txt` com uma senha de teste em uma linha. O Git ignora esse arquivo.

No primeiro terminal, inicie o servidor.

```text
go run ./cmd/rc2 serve -addr 127.0.0.1:9000 -dir data/server -password-file password.txt
```

No segundo, liste os arquivos, envie um arquivo de exemplo e baixe-o.

```text
go run ./cmd/rc2 list -addr 127.0.0.1:9000 -password-file password.txt
go run ./cmd/rc2 upload -addr 127.0.0.1:9000 -file README.md -name exemplo.md -password-file password.txt
go run ./cmd/rc2 download -addr 127.0.0.1:9000 -name exemplo.md -file recebido.md -password-file password.txt
```

Use nomes novos ao repetir o teste. Para executar a validação automatizada, rode `go run ./cmd/validate -cross`.

### Executar pelo navegador

Com o servidor UDP em execução, abra outro terminal na raiz do projeto e rode:

```text
go run ./cmd/web
```

Acesse [http://127.0.0.1:8081](http://127.0.0.1:8081) e informe o endereço e a senha do servidor. A interface permite listar, enviar e baixar arquivos, além de selecionar réplicas da mini-CDN. Veja o [guia da interface](docs/INTERFACE.md).

## Etapas do projeto

| Etapa | Documentação |
|---|---|
| 1 | [Protocolo e janela deslizante](docs/PROTOCOLO.md) |
| 2 | [Operações do cliente e do servidor](docs/ETAPA2.md) |
| 3 | [Robustez, métricas e validação](docs/ETAPA3.md) |
| 4 | [Experimentos com atraso, perda e janelas](docs/ETAPA4.md) |
| 5 | [Mini-CDN em rede local](docs/ETAPA5.md) |
| 6 | [Dashboard de métricas](docs/ETAPA6.md) |

Veja também as [evidências no Wireshark](docs/CAPTURA-S3.md) e o [registro das validações](docs/VALIDACAO.md).
