# CitadelDTL

![banner](./assets/banner.png)

CitadelDTL es una infraestructura Go de custodia institucional con reservas
segregadas, cuentas delegadas, subcuentas operativas y limites por mandato. El
lab esta disenado como CTF de logica economica: todo corre en local, sin
servicios externos, y los tests TypeScript ejecutan fixtures completos contra
el binario Go.

## Componentes

- `src/domain`: tipos de cuenta, mandato, dinero, eventos y snapshots.
- `src/ledger`: libro contable, reservas segregadas y journal.
- `src/mandate`: registro de mandatos, delegaciones y subcuentas.
- `src/settlement`: retiros, liquidaciones y recibos.
- `src/audit`: invariantes de segregacion y controles de mandato.
- `src/api`: servicio de aplicacion usado por CLI y escenarios.
- `src/scenario`: runner determinista para fixtures JSON.

## Uso

Instalar dependencias de tests:

```bash
npm install
```

Ejecutar un fixture:

```bash
go run ./cmd/citadeldtl run tests/fixtures/mandate_bypass.json
```

Listar escenarios incluidos:

```bash
go run ./cmd/citadeldtl list
```

Ejecutar tests:

```bash
npm test
```

Validacion completa:

```bash
npm run ci
```

## CTF

Una cuenta delegada no puede retirar directamente. El fallo esta en la
delegacion a subcuentas operativas: la subcuenta hereda el limite economico del
mandato padre, pero no hereda la restriccion de retirada directa. El resultado
es un bypass de segregacion: fondos bloqueados pueden moverse a una subcuenta
operativa y salir por withdrawal.
