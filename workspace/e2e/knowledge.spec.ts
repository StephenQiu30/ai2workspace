import { expect, test } from '@playwright/test';
import documentIds from '../content/document-ids.json' with { type: 'json' };

test('索引、分类和来源链可以阅读', async ({ page, request }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/read$/);
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    'Ai2sd 知识库',
  );
  await page
    .getByRole('article')
    .getByRole('link', { name: '工作区初始化原始需求', exact: true })
    .click();
  await expect(page).toHaveURL(/\/read\/requirement-01$/);
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    '工作区初始化需求',
  );

  for (const entry of documentIds) {
    const url = entry.id === 'index' ? '/read' : `/read/${entry.id}`;
    const response = await request.get(url);
    expect(response.status(), entry.path).toBe(200);
    expect(await response.text(), entry.path).not.toContain('glowloom/blob');
  }
});

test('中英文搜索返回当前项目内容', async ({ page, request }) => {
  for (const query of ['工作区', 'Workspace', '编写规范']) {
    const response = await request.get('/api/search', { params: { query } });
    expect(response.status()).toBe(200);
    const results = await response.json();
    expect(results.length, query).toBeGreaterThan(0);
    expect(
      results.some((result: { url: string }) => result.url.startsWith('/read')),
    ).toBe(true);
  }

  await page.goto('/read');
  await page
    .getByRole('complementary')
    .getByRole('button', { name: /搜索文档/ })
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('combobox', { name: '搜索文档' }).fill('编写规范');
  await expect(dialog.getByText(/编写规范/).last()).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
});

test('原始 Markdown、LLM 索引与未知页符合路由合同', async ({ request }) => {
  for (const path of [
    '/read.md',
    '/read/requirement-01.md',
    '/llms.txt',
    '/llms-full.txt',
  ]) {
    const response = await request.get(path);
    expect(response.status(), path).toBe(200);
    expect(await response.text(), path).toContain('Ai2sd');
  }
  const negotiated = await request.get('/read/requirement-01', {
    headers: { Accept: 'text/markdown' },
  });
  expect(negotiated.status()).toBe(200);
  expect(await negotiated.text()).toContain('# 工作区初始化需求');
  expect((await request.get('/read/unknown-document')).status()).toBe(404);
  expect((await request.get('/read/unknown-document.md')).status()).toBe(404);
});

test('窄屏可阅读且导航可以打开', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/read');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  const navigation = page.getByRole('button', {
    name: 'Open Sidebar',
    exact: true,
  });
  await navigation.click();
  const sidebar = page.locator('#nd-sidebar-mobile');
  await expect(sidebar).toBeVisible();
  await sidebar.getByRole('button', { name: '原始需求', exact: true }).click();
  await expect(
    sidebar.getByRole('link', { name: /01 工作区初始化需求/ }).first(),
  ).toBeVisible();
  await sidebar
    .getByRole('button', { name: 'Toggle Theme', exact: true })
    .click();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await sidebar
    .getByRole('button', { name: 'Toggle Theme', exact: true })
    .click();
  await expect(page.locator('html')).toHaveClass(/light/);
  await sidebar
    .getByRole('button', { name: 'Close Sidebar', exact: true })
    .click();
  await expect(sidebar).toHaveAttribute('data-state', 'closed');
});

test('设计图表与命令代码可阅读', async ({ page }) => {
  await page.goto('/read/design-01');
  await expect(page.locator('svg[aria-roledescription]').first()).toBeVisible({
    timeout: 15000,
  });
  await page.goto('/read/operations-01');
  await expect(page.locator('pre')).toContainText('pnpm build');
});
