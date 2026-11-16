import { expect, test } from '@playwright/test';

function revisionFrom(url: string) {
  return new URL(url).searchParams.get('revision');
}

test('自用稳定地址、正文、Markdown和搜索采用同一知识修订', async ({
  page,
  request,
}) => {
  await page.goto('/read/requirement-01');
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    '工作区初始化需求',
  );
  const revision = revisionFrom(page.url());
  expect(revision).toMatch(/^[a-f0-9]{64}$/);
  const markdown = await request.get(
    `/read/requirement-01.md?revision=${revision}`,
  );
  expect(markdown.status()).toBe(200);
  expect(markdown.headers()['x-knowledge-revision']).toBe(revision);
  expect(await markdown.text()).toContain('# 工作区初始化需求');
  const search = await request.get('/api/search', {
    params: { query: '工作区', revision: revision!, space: 'self' },
  });
  expect(search.status()).toBe(200);
  expect(search.headers()['x-knowledge-revision']).toBe(revision);
  expect((await search.json()).length).toBeGreaterThan(0);
  const llms = await request.get(`/llms-full.txt?revision=${revision}`);
  expect(llms.headers()['x-knowledge-revision']).toBe(revision);
  expect(await llms.text()).toContain('AGENTS.md');
});

test('冷加载后搜索第一次点击即可打开官方对话框', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  // A fresh context starts without cached lazy search chunks. Do not retry the click.
  await page.goto('/read');
  const trigger = page
    .getByRole('complementary')
    .getByRole('button', { name: /搜索文档/ });
  await trigger.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('combobox', { name: '搜索文档' }).fill('编写规范');
  await expect(dialog.getByText(/编写规范/).last()).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
  expect(errors).toEqual([]);
});

test('空间配置、治理和根规范通过绑定呈现', async ({ page }) => {
  await page.goto('/spaces');
  await expect(page).toHaveURL(/\/spaces\/self/);
  await expect(page.getByRole('link', { name: '创建空间声明' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: '治理问题' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '任务与交接' })).toBeVisible();
  await page.getByRole('link', { name: '编辑空间声明' }).click();
  await expect(
    page.getByRole('textbox', { name: /workspace.yaml/ }),
  ).toContainText('schemaVersion: 1');
  await expect(
    page.getByRole('button', { name: /运行|执行检查|启动任务/ }),
  ).toHaveCount(0);
});

test('内容协商、未知页面和越界来源遵守阅读合同', async ({ request }) => {
  const negotiated = await request.get('/read/requirement-01', {
    headers: { Accept: 'text/markdown' },
  });
  expect(negotiated.status()).toBe(200);
  expect(await negotiated.text()).toContain('# 工作区初始化需求');
  for (const url of [
    '/read/unknown-document',
    '/read/unknown-document.md',
    '/api/search?space=..%2F..',
    '/api/spaces/self/assets/knowledge/..%2F..%2F.env',
  ]) {
    expect((await request.get(url)).status(), url).toBe(404);
  }
});

test('窄屏阅读、目录、图表和主题切换', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/read/design-01');
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await page.getByRole('button', { name: 'Open Sidebar', exact: true }).click();
  const sidebar = page.locator('#nd-sidebar-mobile');
  await expect(sidebar).toBeVisible();
  await sidebar
    .getByRole('button', { name: 'Toggle Theme', exact: true })
    .click();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await sidebar
    .getByRole('button', { name: 'Close Sidebar', exact: true })
    .click();
  await expect(page.locator('svg[aria-roledescription]').first()).toBeVisible({
    timeout: 15000,
  });
});

test('两个空间的同名ID、搜索和安全普通Markdown互相隔离', async ({
  page,
  request,
}) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不注入测试空间');
  await page.goto('/spaces/second/read/knowledge/requirement-01');
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    'Second space',
  );
  const diagram = page.getByRole('img', { name: 'diagram' });
  await expect(diagram).toBeVisible();
  await diagram.scrollIntoViewIfNeeded();
  await expect
    .poll(() =>
      diagram.evaluate((image) => (image as HTMLImageElement).naturalWidth),
    )
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => '__markdownExecuted' in globalThis)).toBe(
    false,
  );
  const revision = revisionFrom(page.url())!;
  const response = await request.get('/api/search', {
    params: { query: 'FixtureOnlySecondSpace', space: 'second', revision },
  });
  const results = await response.json();
  expect(results.length).toBeGreaterThan(0);
  expect(
    results.every((item: { url: string }) =>
      item.url.startsWith('/spaces/second/read/'),
    ),
  ).toBe(true);
  const self = await request.get('/api/search', {
    params: { query: 'FixtureOnlySecondSpace', space: 'self' },
  });
  expect(await self.json()).toEqual([]);
  expect(
    (
      await request.get(
        `/api/spaces/second/assets/knowledge/escape.png?revision=${revision}`,
      )
    ).status(),
  ).toBe(404);
});

test('知识变化和来源失联仍可读取固定旧修订', async ({ page, request }) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改资料');
  const { readFile, writeFile, rename } = await import('node:fs/promises');
  const path = await import('node:path');
  const notes = path.join(
    process.env.AI2WORKSPACE_TEST_REGISTRY!,
    'fixture-notes',
  );
  const document = path.join(notes, 'one.md');
  const original = await readFile(document, 'utf8');
  await page.goto('/spaces/second/read/knowledge/requirement-01');
  const revision = revisionFrom(page.url())!;
  await writeFile(document, '# Updated source\n\nNewRevisionOnly\n');
  try {
    await page.reload();
    await expect(page.getByRole('heading', { level: 1 })).toContainText(
      'Second space',
    );
    await expect(page.getByText('正在阅读旧修订')).toBeVisible();
    const text = await request.get(
      `/spaces/second/read/knowledge/requirement-01.md?revision=${revision}`,
    );
    expect(text.headers()['x-knowledge-revision']).toBe(revision);
    expect(await text.text()).toContain('FixtureOnlySecondSpace');
    await rename(notes, `${notes}-offline`);
    try {
      await page.reload();
      await expect(page.getByRole('heading', { level: 1 })).toContainText(
        'Second space',
      );
      await expect(page.getByText('正在阅读旧修订')).toBeVisible();
    } finally {
      await rename(`${notes}-offline`, notes);
    }
  } finally {
    await writeFile(document, original);
  }
});

test('项目模块入口不创建通用空间，配置保存和并发冲突不执行工程命令', async ({
  page,
}) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改配置');
  const { readFile, writeFile, access } = await import('node:fs/promises');
  const path = await import('node:path');
  const registry = process.env.AI2WORKSPACE_TEST_REGISTRY!;
  const marker = path.join(registry, 'must-not-run');
  const config = path.join(registry, 'self', 'workspace.yaml');
  const original = await readFile(config, 'utf8');
  try {
    await page.goto('/spaces/new');
    await expect(page).toHaveURL(/\/spaces\/self\/settings/);
    await expect(
      page.getByRole('textbox', { name: '空间目录标识' }),
    ).toHaveCount(0);
    const editor = page.getByRole('textbox', { name: /workspace.yaml/ });
    await editor.fill(original.replace('profile: generic', 'profile: invalid'));
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText('未保存');
    expect(await readFile(config, 'utf8')).toBe(original);
    const command =
      "require('node:fs').writeFileSync(" +
      JSON.stringify(marker) +
      ", 'unexpected')";
    const updated = original
      .replace('ai2workspace 自用', '当前项目配置测试')
      .replace(
        'projects: []',
        'projects:\n  - id: app\n    binding: notes\n    inputExclude: []',
      )
      .replace(
        'checks: []',
        'checks:\n  - id: never-run\n    projectId: app\n    cwd: .\n    argv: [' +
          JSON.stringify(process.execPath) +
          ', "-e", ' +
          JSON.stringify(command) +
          ']\n    required: true',
      );
    await editor.fill(updated);
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText('已保存');
    expect(await readFile(config, 'utf8')).toBe(updated);
    await page.reload();
    await expect(editor).toHaveValue(updated);
    const external = original.replace('ai2workspace 自用', '外部修改');
    await writeFile(config, external);
    await editor.fill(original);
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText(
      '配置已被其他操作更新',
    );
    expect(await readFile(config, 'utf8')).toBe(external);
    await expect(access(marker)).rejects.toThrow();
  } finally {
    await writeFile(config, original);
  }
});

test('语义无效声明被拒绝且已有破损配置可以修复', async ({ page }) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改配置');
  const { readFile, writeFile } = await import('node:fs/promises');
  const path = await import('node:path');
  const config = path.join(
    process.env.AI2WORKSPACE_TEST_REGISTRY!,
    'second',
    'workspace.yaml',
  );
  const original = await readFile(config, 'utf8');
  try {
    await page.goto('/spaces/second/settings');
    const editor = page.getByRole('textbox', { name: /workspace.yaml/ });
    const duplicate = original.replace(
      'projects: []',
      "  - id: knowledge\n    binding: notes\n    kind: knowledge\n    include: ['**/*.md']\n    exclude: []\n    writePolicy: read-only\nprojects: []",
    );
    await editor.fill(duplicate);
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText('未保存');
    expect(await readFile(config, 'utf8')).toBe(original);
    const unknownProject = original
      .replace(
        'projects: []',
        'projects: []\nchecks:\n  - id: check\n    projectId: missing\n    cwd: .\n    argv: [node, --version]\n    timeoutMs: 1000\n    required: true',
      )
      .replace(/checks: \[\]\s*$/, '');
    await editor.fill(unknownProject);
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText('未保存');
    expect(await readFile(config, 'utf8')).toBe(original);
    await writeFile(config, duplicate);
    await page.reload();
    await expect(editor).toHaveValue(duplicate);
    await editor.fill(original);
    await page.getByRole('button', { name: '保存声明' }).click();
    await expect(page.getByRole('status')).toContainText('已保存');
    expect(await readFile(config, 'utf8')).toBe(original);
  } finally {
    await writeFile(config, original);
  }
});

test('固定知识修订也固定图片字节，来源失联仍可读取旧图', async ({
  page,
  request,
}) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改资料');
  const { readFile, writeFile, rename } = await import('node:fs/promises');
  const path = await import('node:path');
  const notes = path.join(
    process.env.AI2WORKSPACE_TEST_REGISTRY!,
    'fixture-notes',
  );
  const image = path.join(notes, 'diagram.png');
  const original = await readFile(image);
  await page.goto('/spaces/second/read/knowledge/requirement-01');
  const revision = revisionFrom(page.url())!;
  const src = await page
    .getByRole('img', { name: 'diagram' })
    .getAttribute('src');
  const renderedUrl = new URL(src!, 'http://localhost');
  const asset =
    renderedUrl.pathname === '/_next/image'
      ? renderedUrl.searchParams.get('url')!
      : src!;
  expect(new URL(asset, 'http://localhost').searchParams.get('revision')).toBe(
    revision,
  );
  await writeFile(image, Buffer.from('different image bytes'));
  try {
    const before = await request.get(asset);
    expect(before.status()).toBe(200);
    expect(await before.body()).toEqual(original);
    await rename(notes, `${notes}-offline`);
    try {
      const offline = await request.get(asset);
      expect(offline.status()).toBe(200);
      expect(await offline.body()).toEqual(original);
    } finally {
      await rename(`${notes}-offline`, notes);
    }
  } finally {
    await writeFile(image, original);
  }
});

test('遗留空间重复身份不会污染当前项目，重复登记读取仍被阻止', async ({
  page,
  request,
}) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改配置');
  const { readFile, writeFile, mkdir, rm, access } =
    await import('node:fs/promises');
  const path = await import('node:path');
  const registry = process.env.AI2WORKSPACE_TEST_REGISTRY!;
  const original = await readFile(
    path.join(registry, 'second', 'workspace.yaml'),
    'utf8',
  );
  await page.goto('/spaces/new');
  await expect(page).toHaveURL(/\/spaces\/self\/settings/);
  await expect(page.getByRole('textbox', { name: '空间目录标识' })).toHaveCount(
    0,
  );
  await expect(access(path.join(registry, 'duplicate-new'))).rejects.toThrow();
  const collision = path.join(registry, 'duplicate-existing');
  await mkdir(collision);
  try {
    await writeFile(path.join(collision, 'workspace.yaml'), original);
    await writeFile(
      path.join(collision, 'workspace.local.yaml'),
      await readFile(path.join(registry, 'second', 'workspace.local.yaml')),
    );
    expect(
      (await request.get('/api/search?space=second&query=Second')).status(),
    ).toBe(404);
    expect(
      (
        await request.get('/spaces/second/read/knowledge/requirement-01')
      ).status(),
    ).toBe(404);
    await page.goto('/spaces');
    await expect(page).toHaveURL(/\/spaces\/self/);
    await expect(
      page.getByRole('heading', { name: '任务与交接' }),
    ).toBeVisible();
  } finally {
    await rm(collision, { recursive: true });
  }
});

test('自用知识按职责分类导航，规则独立呈现并保留历史文档地址', async ({
  page,
}) => {
  await page.goto('/read');
  const sidebar = page.getByRole('complementary');
  for (const name of [
    '产品',
    '架构',
    '使用指南',
    '开发',
    '历史归档',
    '适用规范',
  ]) {
    await expect(
      sidebar.getByRole('button', { name, exact: true }),
    ).toBeVisible();
  }
  await expect(
    sidebar.getByRole('button', { name: 'Knowledge', exact: true }),
  ).toHaveCount(0);
  await page.goto('/read/design-01');
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    '工作区初始化设计',
  );
  await page.goto('/read/architecture-ownership');
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    '职责与文档归属',
  );
  await expect(page.getByRole('article')).toContainText('Harness');
});

test('网页目录标题与正文使用同一固定知识修订', async ({ page }) => {
  test.skip(!!process.env.WORKSPACE_TEST_URL, '外部服务不修改资料');
  const { mkdir, writeFile, rm } = await import('node:fs/promises');
  const path = await import('node:path');
  const chapter = path.join(
    process.env.AI2WORKSPACE_TEST_REGISTRY!,
    'fixture-notes',
    'chapter',
  );
  await mkdir(chapter);
  await writeFile(path.join(chapter, 'intro.md'), '# Chapter fixture');
  await writeFile(
    path.join(chapter, 'meta.json'),
    JSON.stringify({ title: '原目录分类', pages: ['intro'] }),
  );
  try {
    await page.goto('/spaces/second/read/knowledge/requirement-01');
    const oldUrl = page.url();
    const sidebar = page.getByRole('complementary');
    await expect(
      sidebar.getByRole('button', { name: '原目录分类', exact: true }),
    ).toBeVisible();
    await writeFile(
      path.join(chapter, 'meta.json'),
      JSON.stringify({ title: '新目录分类', pages: ['intro'] }),
    );
    await page.goto('/spaces/second/read/knowledge/requirement-01');
    await expect(
      sidebar.getByRole('button', { name: '新目录分类', exact: true }),
    ).toBeVisible();
    await page.goto(oldUrl);
    await expect(
      sidebar.getByRole('button', { name: '原目录分类', exact: true }),
    ).toBeVisible();
    await expect(
      sidebar.getByRole('button', { name: '新目录分类', exact: true }),
    ).toHaveCount(0);
  } finally {
    await rm(chapter, { recursive: true });
  }
});

test('配置类型指南可跳转到已接入的空间及交接模板说明', async ({ page }) => {
  await page.goto('/read/guide-profile-generic');
  await page
    .getByRole('article')
    .getByRole('link', { name: '空间模板', exact: true })
    .click();
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    '空间配置样例',
  );
  await page.goto('/read/guide-profile-development');
  await page
    .getByRole('article')
    .getByRole('link', { name: 'Harness 模板', exact: true })
    .click();
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    'Harness 输入模板',
  );
});
