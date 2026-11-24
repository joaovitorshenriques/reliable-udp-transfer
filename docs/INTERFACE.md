# Interface de transferência

Execute na raiz do projeto:

```text
go run ./cmd/web
```

Abra <http://127.0.0.1:8081>. A interface roda na máquina do cliente e aceita apenas conexões HTTP locais. Use `-port` para alterar a porta HTTP.

## Transferência direta

Inicie o servidor UDP conforme o [guia cliente-servidor](ETAPA2.md).
Na página, informe o IP e a porta do servidor e a senha configurada.
Clique em **Listar arquivos** e escolha **Baixar**, ou selecione um arquivo em **Enviar arquivo**. O nome remoto é opcional. O resumo mostra duração da transferência UDP, goodput e SHA-256.

## Mini-CDN

Inicie uma réplica em cada máquina conforme o [guia da mini-CDN](ETAPA5.md).
Informe os dois endereços UDP e a senha comum. **Medir RTT e selecionar** faz cinco sondagens por réplica, mostra as medianas elegíveis e lista os arquivos da réplica escolhida. O botão **Baixar** usa a réplica dessa listagem.

Use a mesma rede de teste isolada da CLI, pois a senha do protocolo UDP trafega em texto claro. A tela executa uma operação por vez. Depois de um upload, o servidor ainda pode responder ocupado enquanto confirma o encerramento do recebimento. Aguarde alguns segundos antes de repetir.

## Voltar à CLI

Encerre a interface com Ctrl+C e use `go run ./cmd/rc2` conforme os guias. O [dashboard de métricas](ETAPA6.md) continua disponível separadamente.
