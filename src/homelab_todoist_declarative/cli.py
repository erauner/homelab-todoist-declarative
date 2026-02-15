import typer


app = typer.Typer(name="todoist-decl", no_args_is_help=True)


@app.command()
def validate(file: str = typer.Option("todoist.yaml", "--file", "-f")) -> None:
    """Validate desired-state configuration file (schema + invariants)."""
    raise typer.Exit(code=0)


@app.command()
def plan(file: str = typer.Option("todoist.yaml", "--file", "-f")) -> None:
    """Show the changes that would be applied (no mutation)."""
    raise typer.Exit(code=0)


@app.command()
def apply(file: str = typer.Option("todoist.yaml", "--file", "-f")) -> None:
    """Apply desired state to Todoist (mutating)."""
    raise typer.Exit(code=0)


@app.command()
def import_config(out: str = typer.Option("todoist.yaml", "--out")) -> None:
    """Export current Todoist config as desired-state YAML."""
    raise typer.Exit(code=0)


if __name__ == "__main__":
    app()

