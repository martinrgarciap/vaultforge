import { Link } from "react-router";
import {
  FiCheckCircle,
  FiCode,
  FiEye,
  FiKey,
  FiLock,
  FiShield,
  FiSliders,
  FiXCircle,
} from "react-icons/fi";
import { FaGithub, FaLinkedinIn } from "react-icons/fa";
import {
  SiGo,
  SiPostgresql,
  SiReact,
  SiReactrouter,
  SiRedis,
  SiRust,
  SiTypescript,
  SiVite,
  SiWebassembly,
} from "react-icons/si";

import { useAuth } from "../auth/useAuth";

export function HomePage() {
  const { status } = useAuth();
  const isAuthenticated = status === "authenticated";

  return (
    <div className="home-shell">
      <section className="home-intro-card">
        <div className="home-intro-copy">
          <h1>VaultForge</h1>

          <p className="home-lead">
            A backend-first developer secrets vault that demonstrates secure API
            design, Rust service integration, browser-side encryption, and
            ciphertext-only storage for vault items.
          </p>

          <div className="home-actions">
            {isAuthenticated ? (
              <Link className="primary-button" to="/vaults">
                Open Vaults
              </Link>
            ) : (
              <Link className="primary-button" to="/register">
                Create demo account
              </Link>
            )}

            {isAuthenticated ? null : (
              <Link className="secondary-button" to="/login">
                Sign in
              </Link>
            )}

            <Link
              className="secondary-button password-generator-hero-link"
              to="/password-generator"
            >
              Password Generator
            </Link>
          </div>

          <div className="home-social-links" aria-label="Project links">
            <a
              className="home-social-link"
              href="https://github.com/martinrgarciap/vaultforge"
              target="_blank"
              rel="noreferrer"
              aria-label="Open VaultForge GitHub repository"
            >
              <FaGithub aria-hidden="true" />
            </a>

            <a
              className="home-social-link"
              href="https://www.linkedin.com/in/martin-garcia-prieto/"
              target="_blank"
              rel="noreferrer"
              aria-label="Open Martin Garcia Prieto LinkedIn profile"
            >
              <FaLinkedinIn aria-hidden="true" />
            </a>
          </div>
        </div>

        <div
          className="home-stack-panel"
          aria-label="VaultForge technology stack"
        >
          <h2>Built With</h2>

          <div className="stack-section">
            <p className="stack-group-label">Backend</p>
            <div className="stack-tech-grid backend-tech-grid">
              <span className="stack-tech-card go-tech-card">
                <SiGo aria-hidden="true" />
                <span>Go</span>
              </span>
              <span className="stack-tech-card rust-tech-card">
                <SiRust aria-hidden="true" />
                <span>Rust</span>
              </span>
              <span className="stack-tech-card postgres-tech-card">
                <SiPostgresql aria-hidden="true" />
                <span>PostgreSQL</span>
              </span>
              <span className="stack-tech-card redis-tech-card">
                <SiRedis aria-hidden="true" />
                <span>Redis</span>
              </span>
            </div>
          </div>

          <div className="stack-section">
            <p className="stack-group-label">Frontend</p>
            <div className="stack-tech-grid frontend-tech-grid">
              <span className="stack-tech-card react-tech-card">
                <SiReact aria-hidden="true" />
                <span>React</span>
              </span>
              <span className="stack-tech-card typescript-tech-card">
                <SiTypescript aria-hidden="true" />
                <span>TypeScript</span>
              </span>
              <span className="stack-tech-card vite-tech-card">
                <SiVite aria-hidden="true" />
                <span>Vite</span>
              </span>
              <span className="stack-tech-card wasm-tech-card">
                <SiWebassembly aria-hidden="true" />
                <span>WASM</span>
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className="system-work-card">
        <h2>How VaultForge Works</h2>

        <div className="flow-grid">
          <article className="flow-card auth-flow-card">
            <div className="flow-title">
              <FiCode aria-hidden="true" />
              <h3>Authentication Flow</h3>
            </div>

            <div className="flow-content">
              <div className="flow-steps auth-steps">
                <div className="flow-step browser-step">
                  <span className="flow-step-icon browser-icon">
                    <FiCode aria-hidden="true" />
                  </span>
                  <strong>Browser</strong>
                  <span>Email + password</span>
                </div>
                <div className="flow-arrow" aria-hidden="true">
                  →
                </div>
                <div className="flow-step go-step">
                  <span className="flow-step-icon go-icon">
                    <SiGo aria-hidden="true" />
                  </span>
                  <strong>Go API</strong>
                  <span>Authentication endpoints</span>
                </div>
                <div className="flow-arrow" aria-hidden="true">
                  →
                </div>
                <div className="flow-step rust-step">
                  <span className="flow-step-icon rust-icon">
                    <SiRust aria-hidden="true" />
                  </span>
                  <strong>Rust hash service</strong>
                  <span>Argon2id hash / verify</span>
                </div>
              </div>

              <div className="data-store-row">
                <div className="data-store-card postgres-step">
                  <span className="flow-step-icon postgres-icon">
                    <SiPostgresql aria-hidden="true" />
                  </span>
                  <strong>PostgreSQL</strong>
                  <span>Users and sessions</span>
                </div>
                <div className="data-store-card redis-step">
                  <span className="flow-step-icon redis-icon">
                    <SiRedis aria-hidden="true" />
                  </span>
                  <strong>Redis</strong>
                  <span>Rate limits and lockouts</span>
                </div>
              </div>
            </div>

            <div className="benefit-callout auth-benefit">
              <FiShield aria-hidden="true" />
              <p>
                <strong>Benefit:</strong> account passwords are hashed and
                verified through the Rust service, while Redis helps protect the
                auth surface with rate limits and lockouts.
              </p>
            </div>
          </article>

          <article className="flow-card vault-flow-card">
            <div className="flow-title">
              <FiLock aria-hidden="true" />
              <h3>Vault Secret Flow</h3>
            </div>

            <div className="flow-content">
              <div className="vault-step-rows">
                <div className="vault-step-row">
                  <div className="flow-step vault-step">
                    <span className="flow-step-icon vault-icon">
                      <FiKey aria-hidden="true" />
                    </span>
                    <strong>Vault passphrase</strong>
                    <span>Used only in browser</span>
                  </div>

                  <div className="flow-arrow" aria-hidden="true">
                    →
                  </div>

                  <div className="flow-step wasm-step">
                    <span className="flow-step-icon wasm-icon">
                      <SiWebassembly aria-hidden="true" />
                    </span>
                    <strong>Rust WASM crypto</strong>
                    <span>Encrypts item payloads locally</span>
                  </div>
                </div>

                <div className="vault-step-row vault-step-row-second">
                  <div
                    className="flow-arrow vault-row-arrow"
                    aria-hidden="true"
                  >
                    →
                  </div>

                  <div className="flow-step go-step">
                    <span className="flow-step-icon go-icon">
                      <SiGo aria-hidden="true" />
                    </span>
                    <strong>Go API</strong>
                    <span>Receives encrypted envelope only</span>
                  </div>

                  <div className="flow-arrow" aria-hidden="true">
                    →
                  </div>

                  <div className="flow-step postgres-step">
                    <span className="flow-step-icon postgres-icon">
                      <SiPostgresql aria-hidden="true" />
                    </span>
                    <strong>PostgreSQL</strong>
                    <span>Ciphertext and crypto metadata</span>
                  </div>
                </div>
              </div>
            </div>

            <div className="benefit-callout vault-benefit">
              <FiShield aria-hidden="true" />
              <p>
                <strong>Benefit:</strong> the vault passphrase never needs to be
                stored by the API. Secrets are encrypted in the browser first,
                so the backend stores ciphertext instead of decrypted values.
              </p>
            </div>
          </article>

          <article className="flow-card password-flow-card">
            <div className="flow-title">
              <FiSliders aria-hidden="true" />
              <h3>Password Generator Flow</h3>
            </div>

            <div className="flow-content">
              <div className="flow-steps password-steps">
                <div className="flow-step browser-step">
                  <span className="flow-step-icon browser-icon">
                    <FiSliders aria-hidden="true" />
                  </span>
                  <strong>Browser</strong>
                  <span>Choose length, symbols, and exclusions</span>
                </div>
                <div className="flow-arrow" aria-hidden="true">
                  →
                </div>
                <div className="flow-step go-step">
                  <span className="flow-step-icon go-icon">
                    <SiGo aria-hidden="true" />
                  </span>
                  <strong>Go API</strong>
                  <span>Public password endpoints</span>
                </div>
                <div className="flow-arrow" aria-hidden="true">
                  →
                </div>
                <div className="flow-step rust-step">
                  <span className="flow-step-icon rust-icon">
                    <SiRust aria-hidden="true" />
                  </span>
                  <strong>Rust password service</strong>
                  <span>Generates passwords and strength results</span>
                </div>
                <div className="flow-arrow" aria-hidden="true">
                  →
                </div>
                <div className="flow-step password-result-step">
                  <span className="flow-step-icon password-result-icon">
                    <FiCheckCircle aria-hidden="true" />
                  </span>
                  <strong>Password + insight</strong>
                  <span>Returned to the browser only</span>
                </div>
              </div>
            </div>

            <div className="benefit-callout password-benefit">
              <FiShield aria-hidden="true" />
              <p>
                <strong>Benefit:</strong> password tools are public and separate
                from vault encryption, while still using a dedicated Rust
                service behind the Go API.
              </p>
            </div>
          </article>
        </div>
      </section>

      <section className="home-summary-grid">
        <article className="home-summary-card security-summary-card">
          <span className="summary-icon security-icon">
            <FiShield aria-hidden="true" />
          </span>
          <div>
            <h2>Security model</h2>
            <p>Account authentication and vault unlocking are separate.</p>
          </div>
          <ul className="summary-check-list">
            <li>
              <FiCheckCircle aria-hidden="true" />
              <span>
                Account passwords are hashed through the Rust service.
              </span>
            </li>
            <li>
              <FiCheckCircle aria-hidden="true" />
              <span>Vault passphrases never leave the browser.</span>
            </li>
            <li>
              <FiCheckCircle aria-hidden="true" />
              <span>Vault items are encrypted before reaching the API.</span>
            </li>
          </ul>
        </article>

        <article className="home-summary-card visibility-summary-card">
          <span className="summary-icon visibility-icon">
            <FiEye aria-hidden="true" />
          </span>
          <div>
            <h2>Visibility boundary</h2>
            <p>The API stores encrypted envelopes, not decrypted secrets.</p>
          </div>
          <div className="api-boundary-grid">
            <div>
              <h3>API can see</h3>
              <ul className="boundary-list can-see-list">
                <li>
                  <FiCheckCircle aria-hidden="true" />
                  Account email and session info
                </li>
                <li>
                  <FiCheckCircle aria-hidden="true" />
                  Vault IDs and ownership checks
                </li>
                <li>
                  <FiCheckCircle aria-hidden="true" />
                  Encrypted item envelopes
                </li>
              </ul>
            </div>
            <div>
              <h3>API cannot see</h3>
              <ul className="boundary-list cannot-see-list">
                <li>
                  <FiXCircle aria-hidden="true" />
                  Vault passphrase
                </li>
                <li>
                  <FiXCircle aria-hidden="true" />
                  Unwrapped vault key
                </li>
                <li>
                  <FiXCircle aria-hidden="true" />
                  Decrypted secrets or notes
                </li>
              </ul>
            </div>
          </div>
        </article>

        <article className="home-summary-card project-summary-card">
          <span className="summary-icon stack-icon">
            <FiCode aria-hidden="true" />
          </span>
          <div>
            <h2>Project highlights</h2>
            <p>Security-focused full-stack engineering pieces.</p>
          </div>
          <ul className="project-highlight-list">
            <li>
              <SiGo aria-hidden="true" />
              <span>Go REST API, middleware, auth, validation, OpenAPI</span>
            </li>
            <li>
              <SiRedis aria-hidden="true" />
              <span>Redis rate limits, login lockouts, readiness checks</span>
            </li>
            <li>
              <SiRust aria-hidden="true" />
              <span>Rust gRPC hashing and Rust WASM browser crypto</span>
            </li>
            <li>
              <SiReact aria-hidden="true" />
              <span>React + TypeScript workflows for secure vault UX</span>
            </li>
            <li>
              <SiReactrouter aria-hidden="true" />
              <span>React Router client flows and protected navigation</span>
            </li>
          </ul>
        </article>
      </section>

      <section
        className="password-tool-callout"
        aria-labelledby="password-tool-title"
      >
        <div>
          <p className="page-kicker">Public tool</p>
          <h2 id="password-tool-title">Try the Password Generator</h2>
          <p>
            Create strong passwords and check strength without creating an
            account. This tool is separate from vault encryption and storage.
          </p>
        </div>

        <Link className="primary-button" to="/password-generator">
          Open Password Generator
        </Link>
      </section>
    </div>
  );
}
