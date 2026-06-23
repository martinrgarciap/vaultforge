import AxeBuilder from "@axe-core/playwright";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "@playwright/test";

interface Credentials {
  email: string;
  password: string;
}

interface BrowserPrivacySnapshot {
  localStorageEntryCount: number;
  sessionStorageEntryCount: number;
  indexedDBInspectionSupported: boolean;
  indexedDBCount: number;
  url: string;
}

interface BrowserOutputCapture {
  consoleMessages: string[];
  pageErrors: string[];
}

const responsiveViewports = [
  {
    name: "phone",
    width: 390,
    height: 844,
  },
  {
    name: "tablet",
    width: 768,
    height: 1024,
  },
  {
    name: "desktop",
    width: 1440,
    height: 900,
  },
] as const;

async function expectNoDocumentOverflow(page: Page): Promise<void> {
  await expect
    .poll(async () =>
      page.evaluate(() => {
        const root = document.documentElement;

        return root.scrollWidth <= root.clientWidth + 1;
      }),
    )
    .toBe(true);
}

async function readBrowserPrivacySnapshot(
  page: Page,
): Promise<BrowserPrivacySnapshot> {
  return page.evaluate(async () => {
    const indexedDBInspectionSupported =
      typeof indexedDB.databases === "function";

    const databases = indexedDBInspectionSupported
      ? await indexedDB.databases()
      : [];

    return {
      localStorageEntryCount: window.localStorage.length,
      sessionStorageEntryCount: window.sessionStorage.length,
      indexedDBInspectionSupported,
      indexedDBCount: databases.length,
      url: window.location.href,
    };
  });
}

async function expectNoSensitiveBrowserPersistence(
  page: Page,
  sensitiveValues: string[],
): Promise<void> {
  const snapshot = await readBrowserPrivacySnapshot(page);

  expect(
    snapshot.localStorageEntryCount,
    "localStorage must remain empty.",
  ).toBe(0);

  expect(
    snapshot.sessionStorageEntryCount,
    "sessionStorage must remain empty.",
  ).toBe(0);

  expect(
    snapshot.indexedDBInspectionSupported,
    "The Chromium smoke test must be able to inspect IndexedDB databases.",
  ).toBe(true);

  expect(
    snapshot.indexedDBCount,
    "VaultForge must not create an IndexedDB database.",
  ).toBe(0);

  let decodedURL = snapshot.url;

  try {
    decodedURL = decodeURIComponent(snapshot.url);
  } catch {
    decodedURL = snapshot.url;
  }

  const urlContainsSensitiveValue = sensitiveValues.some(
    (sensitiveValue) =>
      snapshot.url.includes(sensitiveValue) ||
      decodedURL.includes(sensitiveValue),
  );

  expect(
    urlContainsSensitiveValue,
    "The browser URL must not contain synthetic sensitive values.",
  ).toBe(false);
}

function captureBrowserOutput(page: Page, output: BrowserOutputCapture): void {
  page.on("console", (message) => {
    output.consoleMessages.push(message.text());
  });

  page.on("pageerror", (error) => {
    output.pageErrors.push(error.message);
  });
}

function expectNoSensitiveBrowserOutput(
  output: BrowserOutputCapture,
  sensitiveValues: string[],
): void {
  const capturedOutput = [...output.consoleMessages, ...output.pageErrors].join(
    "\n",
  );

  const outputContainsSensitiveValue = sensitiveValues.some((sensitiveValue) =>
    capturedOutput.includes(sensitiveValue),
  );

  expect(
    outputContainsSensitiveValue,
    "Browser console messages and page errors must not contain synthetic sensitive values.",
  ).toBe(false);
}

async function expectNoAccessibilityViolations(page: Page): Promise<void> {
  const result = await new AxeBuilder({
    page,
  })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();

  const violations = result.violations.map((violation) => ({
    id: violation.id,
    impact: violation.impact,
    targets: violation.nodes.map((node) => node.target),
  }));

  expect(violations).toEqual([]);
}

async function login(page: Page, credentials: Credentials): Promise<void> {
  await page.getByLabel("Email address").fill(credentials.email);

  await page
    .getByLabel("Password", {
      exact: true,
    })
    .fill(credentials.password);

  await page
    .getByRole("button", {
      name: "Sign in",
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: "Your Vaults",
    }),
  ).toBeVisible();
}

async function openEditDialog(page: Page) {
  await page
    .getByRole("button", {
      name: "Edit",
      exact: true,
    })
    .click();

  const dialog = page.getByRole("dialog", {
    name: "Edit Item",
  });

  await expect(dialog).toBeVisible();

  return dialog;
}

async function updateItemTitle(page: Page, title: string): Promise<void> {
  const dialog = await openEditDialog(page);

  await dialog.getByLabel("Edit item title").fill(title);

  await dialog
    .getByRole("button", {
      name: "Save Item",
    })
    .click();
}

async function deleteItem(page: Page): Promise<void> {
  await page
    .getByRole("button", {
      name: "Delete",
      exact: true,
    })
    .click();

  const dialog = page.getByRole("dialog", {
    name: "Delete Item?",
  });

  await expect(dialog).toBeVisible();

  await dialog
    .getByRole("button", {
      name: "Delete",
      exact: true,
    })
    .click();

  await expect(
    page.getByRole("button", {
      name: "Restore",
    }),
  ).toBeVisible();
}

async function loginSecondBrowser(
  browser: Browser,
  origin: string,
  credentials: Credentials,
  itemURL: string,
  browserOutput: BrowserOutputCapture,
): Promise<{
  page: Page;
  close: () => Promise<void>;
}> {
  const context = await browser.newContext();
  const page = await context.newPage();

  captureBrowserOutput(page, browserOutput);

  await page.goto(`${origin}/login`);
  await login(page, credentials);

  await page.goto(itemURL);

  return {
    page,
    close: async () => {
      await context.close();
    },
  };
}

test("real browser workflow crosses the frontend, API, and PostgreSQL boundaries", async ({
  browser,
  page,
}) => {
  const browserOutput: BrowserOutputCapture = {
    consoleMessages: [],
    pageErrors: [],
  };

  captureBrowserOutput(page, browserOutput);

  const runID = `${Date.now()}-${Math.random().toString(16).slice(2)}`;

  const credentials: Credentials = {
    email: `playwright-${runID}@example.test`,
    password: "synthetic Playwright password 2026",
  };

  const vaultName = `E2E Vault ${runID}`;
  const originalTitle = "E2E Login";
  const concurrentTitle = "Updated From Second Session";
  const syntheticItemPassword = "synthetic-password";

  for (const viewport of responsiveViewports) {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    });

    await page.goto("/register");

    await expect(
      page.getByRole("heading", {
        name: "Create your account",
      }),
    ).toBeVisible();

    await expectNoDocumentOverflow(page);
  }

  await page.setViewportSize({
    width: 1440,
    height: 900,
  });

  await page.goto("/register");
  await expectNoAccessibilityViolations(page);

  await page.getByLabel("Email address").fill(credentials.email);

  await page
    .getByLabel("Password", {
      exact: true,
    })
    .fill(credentials.password);

  await page
    .getByLabel("Confirm password", {
      exact: true,
    })
    .fill(credentials.password);

  await page
    .getByRole("button", {
      name: "Create account",
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: "Sign in",
    }),
  ).toBeVisible();

  await expect(page.getByRole("status")).toContainText(
    "Account created. Sign in to continue.",
  );

  await login(page, credentials);

  await expectNoSensitiveBrowserPersistence(page, [credentials.password]);

  const cookies = await page.context().cookies();

  expect(cookies.some((cookie) => cookie.httpOnly)).toBe(true);

  await page
    .getByRole("button", {
      name: "Create Vault",
    })
    .click();

  const vaultDialog = page.getByRole("dialog", {
    name: "Create Vault",
  });

  await vaultDialog.getByLabel("Vault name").fill(vaultName);

  await vaultDialog
    .getByRole("button", {
      name: "Create Vault",
      exact: true,
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: "Vault Details",
    }),
  ).toBeVisible();

  await expect(page.getByText(vaultName)).toBeVisible();

  const vaultURL = page.url();

  await page
    .getByRole("button", {
      name: "Create Item",
    })
    .click();

  const itemDialog = page.getByRole("dialog", {
    name: "Create an item",
  });

  await itemDialog.getByLabel("Item type").selectOption("login");

  await itemDialog.getByLabel("New item title").fill(originalTitle);

  await itemDialog.getByLabel("New item website").fill("https://example.test");

  await itemDialog.getByLabel("New item username").fill("synthetic-user");

  await itemDialog.getByLabel("New item password").fill(syntheticItemPassword);

  await itemDialog
    .getByRole("button", {
      name: "Create item",
      exact: true,
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: "Logins",
    }),
  ).toBeVisible();

  await page
    .getByRole("link", {
      name: `Open ${originalTitle}`,
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: originalTitle,
    }),
  ).toBeVisible();

  await expect(page.getByText(syntheticItemPassword)).toHaveCount(0);

  const showPasswordButton = page.getByRole("button", {
    name: "Show password",
  });

  await expect(showPasswordButton).toBeVisible();

  await expectNoSensitiveBrowserPersistence(page, [
    credentials.password,
    syntheticItemPassword,
  ]);

  await showPasswordButton.click();

  await expect(page.getByText(syntheticItemPassword)).toBeVisible();

  await expectNoSensitiveBrowserPersistence(page, [
    credentials.password,
    syntheticItemPassword,
  ]);

  const origin = new URL(page.url()).origin;

  await page.context().grantPermissions(["clipboard-read", "clipboard-write"], {
    origin,
  });

  const copyPasswordButton = page.getByRole("button", {
    name: "Copy password",
  });

  await copyPasswordButton.click();

  await expect(page.getByTitle("Password copied")).toBeVisible();

  await expectNoSensitiveBrowserPersistence(page, [
    credentials.password,
    syntheticItemPassword,
  ]);

  await page
    .getByRole("button", {
      name: "Hide password",
    })
    .click();

  await expect(page.getByText(syntheticItemPassword)).toHaveCount(0);

  const itemURL = page.url();

  // The access token is memory-only. Reloading must
  // restore authentication through the refresh cookie.
  await page.reload();

  await expect(
    page.getByRole("heading", {
      name: originalTitle,
    }),
  ).toBeVisible();

  await expect(page.getByText(syntheticItemPassword)).toHaveCount(0);

  await expectNoSensitiveBrowserPersistence(page, [
    credentials.password,
    syntheticItemPassword,
  ]);

  const secondBrowser = await loginSecondBrowser(
    browser,
    origin,
    credentials,
    itemURL,
    browserOutput,
  );

  try {
    await expect(
      secondBrowser.page.getByRole("heading", {
        name: originalTitle,
      }),
    ).toBeVisible();

    // Both browsers loaded version 1. The second
    // browser advances the item to version 2.
    await updateItemTitle(secondBrowser.page, concurrentTitle);

    await expect(
      secondBrowser.page.getByRole("heading", {
        name: concurrentTitle,
      }),
    ).toBeVisible();

    // The first browser still submits version 1.
    await updateItemTitle(page, "Stale Browser Update");

    await expect(
      page.getByText(
        "VaultForge will not overwrite a newer item version automatically.",
        {
          exact: false,
        },
      ),
    ).toBeVisible();

    await expect(
      page.getByRole("button", {
        name: "Reload Current Item",
      }),
    ).toBeVisible();

    await page
      .getByRole("button", {
        name: "Reload Current Item",
      })
      .click();

    await expect(
      page.getByRole("heading", {
        name: concurrentTitle,
      }),
    ).toBeVisible();

    await expectNoAccessibilityViolations(page);

    for (const viewport of responsiveViewports) {
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });

      await expect(
        page.getByRole("heading", {
          name: concurrentTitle,
        }),
      ).toBeVisible();

      await expectNoDocumentOverflow(page);
    }

    await page.setViewportSize({
      width: 1440,
      height: 900,
    });

    await deleteItem(page);

    await page
      .getByRole("button", {
        name: "Restore",
      })
      .click();

    await expect(
      page.getByRole("button", {
        name: "Edit",
      }),
    ).toBeVisible();

    await deleteItem(page);

    await page
      .getByRole("button", {
        name: "Delete Permanently",
      })
      .click();

    const permanentDeleteDialog = page.getByRole("dialog", {
      name: "Permanently Delete Item?",
    });

    await permanentDeleteDialog
      .getByRole("button", {
        name: "Delete Permanently",
      })
      .click();

    await expect(
      page.getByRole("heading", {
        name: "Vault Details",
      }),
    ).toBeVisible();

    await page
      .getByRole("link", {
        name: "Sessions",
      })
      .click();

    await expect(
      page.getByRole("heading", {
        name: "Active Sessions",
      }),
    ).toBeVisible();

    await expect(page.getByText("Current Session")).toBeVisible();

    await expect(
      page.getByRole("button", {
        name: "Revoke",
      }),
    ).toBeVisible();

    await expectNoAccessibilityViolations(page);

    const primaryNavigation = page.getByRole("navigation", {
      name: "Primary navigation",
    });

    await primaryNavigation
      .getByRole("button", {
        name: "Log out",
        exact: true,
      })
      .click();

    await expect(
      page.getByRole("heading", {
        name: "Sign in",
      }),
    ).toBeVisible();

    await page.goto(vaultURL);

    await expect(
      page.getByRole("heading", {
        name: "Sign in",
      }),
    ).toBeVisible();

    await expect(page.getByRole("status")).toContainText(
      "Your session is not active. Sign in to continue.",
    );

    expectNoSensitiveBrowserOutput(browserOutput, [
      credentials.password,
      syntheticItemPassword,
    ]);
  } finally {
    await secondBrowser.close();
  }
});
