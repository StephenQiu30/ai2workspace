import {NativeConnection, Worker} from '@temporalio/worker';

async function run(): Promise<void> {
  const connection = await NativeConnection.connect({
    address: process.env.TEMPORAL_ADDRESS || '127.0.0.1:7233',
  });
  try {
    const worker = await Worker.create({
      connection,
      namespace: process.env.TEMPORAL_NAMESPACE || 'default',
      taskQueue: 'ai2workspace',
      workflowsPath: require.resolve('./workflows'),
    });
    await worker.run();
  } finally {
    await connection.close();
  }
}

run().catch((error: unknown) => {
  console.error('Temporal Worker stopped', error);
  process.exitCode = 1;
});
