import { Link } from "@tanstack/react-router";

export function Brand() {
  return (
    <Link to="/" className="mb-4 block px-3 py-2 text-lg font-bold text-foreground">
      vd <span className="text-primary">web</span>
    </Link>
  );
}
