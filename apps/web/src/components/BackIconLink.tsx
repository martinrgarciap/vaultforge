import { Link } from "react-router";

interface BackIconLinkProps {
  to: string;
  label: string;
}

export function BackIconLink({ to, label }: BackIconLinkProps) {
  return (
    <Link className="back-icon-link" to={to} aria-label={label} title={label}>
      <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path
          d="m14.5 5-7 7 7 7"
          fill="none"
          stroke="currentColor"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2"
        />
      </svg>
    </Link>
  );
}
