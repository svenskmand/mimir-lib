import pytest
from job import IntegrationTestConfig, Job, kill_jobs


# Mark test module so that we can run tests by tags
pytestmark = [pytest.mark.default, pytest.mark.job]


@pytest.mark.smoketest
def test__create_batch_job():
    job = Job(job_file='test_job_no_container.yaml',
              config=IntegrationTestConfig(max_retry_attempts=100))
    job.create()
    job.wait_for_state()


@pytest.mark.smoketest
def test__create_job():
    job = Job(config=IntegrationTestConfig(max_retry_attempts=100))
    job.create()
    job.wait_for_state()


@pytest.mark.smoketest
def test__stop_long_running_batch_job_immediately(long_running_job):
    long_running_job.config = IntegrationTestConfig(max_retry_attempts=100)
    long_running_job.job_config.instanceCount = 100
    long_running_job.create()
    long_running_job.wait_for_state(goal_state='RUNNING')

    long_running_job.stop()
    long_running_job.wait_for_state(goal_state='KILLED')


def test__run_failing_job():
    job = Job(job_file='test_job_fail.yaml',
              config=IntegrationTestConfig(max_retry_attempts=100))
    job.create()
    job.wait_for_state(goal_state='FAILED', failed_state='SUCCEEDED')

    results = job.get_task_runs(0)
    assert len(results) == 5
    # TBD uncomment after the ability to fetch logs from mesos in pcluster
    # for i in range(0, len(results)):
    # result = results[i]
    # browse_response = \
    # job.browse_task_sandbox(i, result.runtime.mesosTaskId.value)
    # assert browse_response.hostname
    # assert browse_response.paths


def test__create_a_batch_job_and_restart_jobmgr_completes_jobs(jobmgr):
    job = Job(job_file='test_job_no_container.yaml',
              config=IntegrationTestConfig(max_retry_attempts=100))
    job.create()

    # Restart immediately. That will lave some fraction unallocated and another
    # fraction initialized.
    jobmgr.restart()

    job.wait_for_state()
    kill_jobs([job])


@pytest.mark.smoketest
def test_update_job_increase_instances():
    job = Job(job_file='long_running_job.yaml',
              config=IntegrationTestConfig(max_retry_attempts=100))
    job.create()
    job.wait_for_state(goal_state='RUNNING')

    # job has only 1 task to begin with
    expected_count = 3

    def tasks_count():
        count = 0
        for t in job.get_tasks().values():
            if t.state == 8 or t.state == 9:
                count += 1

        print "total instances running/completed: %d" % count
        return count == expected_count

    job.wait_for_condition(tasks_count)

    # update the job with the new config
    job.update(new_job_file='long_running_job_update_instances.yaml')

    # number of tasks should increase to 4
    expected_count = 4
    job.wait_for_condition(tasks_count)
    job.wait_for_state(goal_state='RUNNING')

    kill_jobs([job])
