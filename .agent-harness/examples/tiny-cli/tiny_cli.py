def greeting(name: str) -> str:
    clean_name = str(name).strip()
    if not clean_name:
        clean_name = "agent"
    return f"hello, {clean_name}"


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Tiny harness example CLI.")
    parser.add_argument("name", nargs="?", default="agent")
    args = parser.parse_args()
    print(greeting(args.name))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

