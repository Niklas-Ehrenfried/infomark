import React, { useCallback, useEffect, useState } from 'react';
import {
  Row,
  Col,
  Card,
  Tag,
  Slider,
  Button,
  Space,
  Skeleton,
  Empty,
  message,
  Avatar,
  List,
  Dropdown,
  Modal,
  Select,
  Typography,
  Popconfirm,
  Alert,
  Radio,
  Statistic,
  Divider,
} from 'antd';

const { Text } = Typography;
import {
  TeamOutlined,
  UserOutlined,
  CrownOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  UserAddOutlined,
  ThunderboltOutlined,
  ClearOutlined,
  CheckCircleOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import agent from '@/api/agent';
import type { Group, GroupBid, Enrollment, Course, AutoAssignResult } from '@/types';
import { useTranslation } from 'react-i18next';

interface GroupsPanelProps {
  courseId: number;
  isStudent: boolean;
  isTutor: boolean;
  // Admins get per-group edit/delete affordances; the handlers live in the
  // parent so navigation/refresh stay centralised.
  canManage?: boolean;
  onEdit?: (groupId: number) => void;
  onDelete?: (groupId: number) => void;
}

// Bids are constrained to 0..10 by the backend (group_requests.go GroupBidRequest).
const BID_MIN = 0;
const BID_MAX = 10;
const DEFAULT_BID = 5;

// GroupsPanel shows every exercise group. Students see their own group
// highlighted and can place a 0..10 preference bid on each group (used by the
// backend's auto-assignment). Tutors additionally see the students enrolled in
// each group.
export const GroupsPanel: React.FC<GroupsPanelProps> = ({
  courseId,
  isStudent,
  isTutor,
  canManage,
  onEdit,
  onDelete,
}) => {
  const { t } = useTranslation();
  const [course, setCourse] = useState<Course | null>(null);
  const [groups, setGroups] = useState<Group[]>([]);
  const [ownGroupId, setOwnGroupId] = useState<number | null>(null);
  const [bids, setBids] = useState<Map<number, number>>(new Map());
  const [members, setMembers] = useState<Map<number, Enrollment[]>>(new Map());
  const [loading, setLoading] = useState(true);
  const [saveAllBidsLoading, setSaveAllBidsLoading] = useState(false);

  // Admin Group Assignment State
  const [autoAssignLoading, setAutoAssignLoading] = useState(false);
  const [autoAssignModalVisible, setAutoAssignModalVisible] = useState(false);
  const [selectedSolver, setSelectedSolver] = useState<'optimal' | 'regret' | 'greedy'>('optimal');
  const [assignResult, setAssignResult] = useState<AutoAssignResult | null>(null);
  const [resultModalVisible, setResultModalVisible] = useState(false);

  const [resetLoading, setResetLoading] = useState(false);
  const [assignModalVisible, setAssignModalVisible] = useState(false);
  const [targetGroupId, setTargetGroupId] = useState<number | null>(null);
  const [courseStudents, setCourseStudents] = useState<Enrollment[]>([]);
  const [selectedStudentUserId, setSelectedStudentUserId] = useState<number | null>(null);
  const [assignLoading, setAssignLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [groupsData, courseData] = await Promise.all([
        agent.Groups.getAll(courseId),
        agent.Courses.get(courseId).catch(() => null),
      ]);
      setGroups(groupsData);
      setCourse(courseData);
      if (courseData?.group_solver) {
        setSelectedSolver(courseData.group_solver);
      }

      if (isStudent) {
        // The student's own group (a one-element list) and their bids.
        const [own, bidList] = await Promise.all([
          agent.Groups.getOwn(courseId).catch(() => [] as Group[]),
          agent.Groups.getBids(courseId).catch(() => [] as GroupBid[]),
        ]);
        setOwnGroupId(own.length > 0 ? own[0].id : null);
        const bidMap = new Map<number, number>();
        bidList.forEach((b) => bidMap.set(b.group_id, b.bid));
        setBids(bidMap);
      }

      if (isTutor || canManage) {
        // Enrolled students per group (tutor/admin only endpoint).
        const memberMap = new Map<number, Enrollment[]>();
        await Promise.all(
          groupsData.map(async (group) => {
            try {
              const enrolled = await agent.Groups.getEnrollments(courseId, group.id);
              memberMap.set(group.id, enrolled);
            } catch {
              memberMap.set(group.id, []);
            }
          }),
        );
        setMembers(memberMap);

        if (canManage) {
          const enrollments = await agent.Courses.getEnrollments(courseId).catch(() => []);
          setCourseStudents(enrollments.filter((e) => e.role === 0));
        }
      }
    } catch {
      message.error(t('messages.updateFailed'));
    } finally {
      setLoading(false);
    }
  }, [courseId, isStudent, isTutor, canManage, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Anti-Gaming Calculations
  const minRequiredPreferences = Math.min(course?.min_group_preferences || 3, groups.length);
  const positiveBidsCount = groups.filter((g) => (bids.get(g.id) ?? DEFAULT_BID) > 0).length;
  const isAntiGamingSatisfied = positiveBidsCount >= minRequiredPreferences;

  const handleSaveAllBids = async () => {
    if (!isAntiGamingSatisfied) {
      message.warning(
        t('group.antiGamingWarning', { min: minRequiredPreferences }),
      );
      return;
    }
    setSaveAllBidsLoading(true);
    try {
      const bidList = groups.map((g) => ({
        group_id: g.id,
        bid: bids.get(g.id) ?? DEFAULT_BID,
      }));
      await agent.Groups.submitBids(courseId, bidList);
      message.success(t('group.prefsSubmitted'));
    } catch (err: any) {
      message.error(err.message || t('group.prefsSubmitFailed'));
    } finally {
      setSaveAllBidsLoading(false);
    }
  };

  const handleRunAutoAssign = async () => {
    setAutoAssignLoading(true);
    try {
      const res = await agent.Groups.autoAssign(courseId, selectedSolver);
      setAssignResult(res);
      setAutoAssignModalVisible(false);
      setResultModalVisible(true);
      await fetchData();
    } catch (err: any) {
      message.error(err.message || t('group.autoAssignFailed'));
    } finally {
      setAutoAssignLoading(false);
    }
  };

  const handleResetAssignments = async () => {
    setResetLoading(true);
    try {
      await agent.Groups.resetAssignments(courseId);
      message.success(t('group.resetSuccess'));
      await fetchData();
    } catch (err: any) {
      message.error(err.message || t('group.resetFailed'));
    } finally {
      setResetLoading(false);
    }
  };

  const handleOpenAssignModal = async (groupId: number) => {
    setTargetGroupId(groupId);
    setSelectedStudentUserId(null);
    setAssignModalVisible(true);
    try {
      const enrollments = await agent.Courses.getEnrollments(courseId);
      setCourseStudents(enrollments.filter((e) => e.role === 0));
    } catch {
      setCourseStudents([]);
    }
  };

  const handleConfirmAssign = async () => {
    if (!targetGroupId || !selectedStudentUserId) return;
    setAssignLoading(true);
    try {
      await agent.Groups.changeEnrollment(courseId, targetGroupId, {
        user_id: selectedStudentUserId,
        group_id: targetGroupId,
      });
      message.success(t('messages.saveSuccess'));
      setAssignModalVisible(false);
      setSelectedStudentUserId(null);
      await fetchData();
    } catch {
      message.error(t('messages.saveFailed'));
    } finally {
      setAssignLoading(false);
    }
  };

  if (loading) return <Skeleton active paragraph={{ rows: 4 }} />;
  if (groups.length === 0) return <Empty description={t('group.groups')} />;

  const isAssigned = isStudent && ownGroupId !== null && ownGroupId !== undefined;
  const displayedGroups = isAssigned
    ? groups.filter((g) => g.id === ownGroupId)
    : groups;

  const targetGroupCap = groups.length > 0 ? Math.ceil(courseStudents.length / groups.length) : 0;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {canManage && (
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <Popconfirm
            title={t('group.resetTitle')}
            description={t('group.resetDesc')}
            onConfirm={handleResetAssignments}
            okText={t('group.resetConfirm')}
            okButtonProps={{ danger: true }}
            cancelText={t('common.cancel')}
          >
            <Button
              danger
              icon={<ClearOutlined />}
              loading={resetLoading}
            >
              {t('group.resetButton')}
            </Button>
          </Popconfirm>
          <Button
            type="primary"
            icon={<ThunderboltOutlined />}
            onClick={() => setAutoAssignModalVisible(true)}
          >
            {t('group.autoAssignButton')}
          </Button>
        </div>
      )}

      {isStudent && !isAssigned && (
        <Alert
          type={isAntiGamingSatisfied ? 'success' : 'warning'}
          showIcon
          icon={<SafetyCertificateOutlined />}
          message={t('group.prefSelectionTitle')}
          description={
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
              <div>
                <p style={{ margin: '0 0 6px 0', fontSize: 13 }}>
                  {t('group.prefSelectionDesc')}
                </p>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <Tag color={isAntiGamingSatisfied ? 'green' : 'orange'}>
                    {t('group.acceptableGroupsCount', { count: positiveBidsCount, min: minRequiredPreferences })}
                  </Tag>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {t('group.antiGamingRule', { min: minRequiredPreferences })}
                  </Text>
                </div>
              </div>
              <Button
                type="primary"
                size="middle"
                loading={saveAllBidsLoading}
                onClick={handleSaveAllBids}
                disabled={!isAntiGamingSatisfied}
              >
                {t('group.saveAllPrefs')}
              </Button>
            </div>
          }
        />
      )}

      {isStudent && isAssigned && (
        <Alert
          type="success"
          showIcon
          message={t('group.assignedGroupTitle')}
          description={t('group.assignedGroupDesc')}
        />
      )}

      <Row gutter={[16, 16]}>
        {displayedGroups.map((group) => {
          const isOwn = ownGroupId === group.id;
          const groupMembers = members.get(group.id) ?? [];
          return (
            <Col xs={24} sm={12} lg={8} key={group.id}>
              <Card
                title={
                  <Space>
                    <TeamOutlined />
                    {`${t('group.group')} ${group.id}`}
                    {isOwn && (
                      <Tag icon={<CrownOutlined />} color="gold">
                        {t('course.myGroup')}
                      </Tag>
                    )}
                  </Space>
                }
                style={isOwn ? { borderColor: '#faad14', borderWidth: 2 } : undefined}
                extra={
                  canManage && (
                    <Dropdown
                      menu={{
                        items: [
                          {
                            key: 'assign',
                            icon: <UserAddOutlined />,
                            label: t('group.assignStudentAction'),
                            onClick: () => handleOpenAssignModal(group.id),
                          },
                          {
                            key: 'edit',
                            icon: <EditOutlined />,
                            label: t('common.edit'),
                            onClick: () => onEdit?.(group.id),
                          },
                          {
                            key: 'delete',
                            icon: <DeleteOutlined />,
                            label: t('common.delete'),
                            danger: true,
                            onClick: () => onDelete?.(group.id),
                          },
                        ],
                      }}
                    >
                      <Button type="text" icon={<MoreOutlined />} aria-label={t('common.actions')} />
                    </Dropdown>
                  )
                }
              >
                <p style={{ minHeight: 44 }}>{group.description}</p>
                <div>
                  <UserOutlined /> {t('group.tutor')}: {group.tutor.first_name}{' '}
                  {group.tutor.last_name}
                </div>

                {isStudent && !isAssigned && (
                  <div style={{ marginTop: 14 }}>
                    <Text strong style={{ fontSize: 13 }}>
                      {t('group.prefBidLabel')}
                    </Text>
                    <div style={{ padding: '0 12px', marginTop: 6, marginBottom: 8 }}>
                      <Slider
                        min={BID_MIN}
                        max={BID_MAX}
                        marks={{
                          0: { label: t('group.cannotAttend'), style: { fontSize: 11, whiteSpace: 'nowrap', transform: 'translateX(0%)' } },
                          5: { label: t('group.neutral'), style: { fontSize: 11, whiteSpace: 'nowrap' } },
                          10: { label: t('group.topChoice'), style: { fontSize: 11, whiteSpace: 'nowrap', transform: 'translateX(-100%)' } },
                        }}
                        value={bids.get(group.id) ?? DEFAULT_BID}
                        onChange={(val) =>
                          setBids((prev) => new Map(prev).set(group.id, val))
                        }
                      />
                    </div>
                  </div>
                )}

                {(isTutor || canManage) && (
                  <List
                    size="small"
                    style={{ marginTop: 14 }}
                    header={
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <strong>
                          {t('group.members')} ({groupMembers.filter((m) => m.role === 0).length})
                        </strong>
                        {canManage && (
                          <Button
                            size="small"
                            type="dashed"
                            icon={<UserAddOutlined />}
                            onClick={() => handleOpenAssignModal(group.id)}
                          >
                            {t('common.add')}
                          </Button>
                        )}
                      </div>
                    }
                    dataSource={groupMembers.filter((m) => m.role === 0)}
                    locale={{
                      emptyText: (
                        <Empty
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                          description={t('group.members')}
                        />
                      ),
                    }}
                    renderItem={(member) => (
                      <List.Item>
                        <List.Item.Meta
                          avatar={
                            <Avatar
                              size="small"
                              icon={<UserOutlined />}
                              src={member.user.avatar_url}
                            />
                          }
                          title={`${member.user.first_name} ${member.user.last_name}`}
                          description={member.user.email}
                        />
                      </List.Item>
                    )}
                  />
                )}
              </Card>
            </Col>
          );
        })}
      </Row>

      {/* Admin Auto-Assign Configuration Modal */}
      <Modal
        title={
          <Space>
            <ThunderboltOutlined style={{ color: '#1677ff' }} />
            <span>{t('group.autoAssignModalTitle')}</span>
          </Space>
        }
        open={autoAssignModalVisible}
        onOk={handleRunAutoAssign}
        onCancel={() => setAutoAssignModalVisible(false)}
        confirmLoading={autoAssignLoading}
        okText={t('group.runAutoAssign')}
        cancelText={t('common.cancel')}
        width={560}
      >
        <Space direction="vertical" size="middle" style={{ width: '100%', marginTop: 12 }}>
          <Alert
            type="info"
            message={t('group.capacityOverview')}
            description={
              <div>
                <div>{t('group.enrolledStudentsLabel')} <strong>{courseStudents.length}</strong></div>
                <div>{t('group.tutorialGroupsLabel')} <strong>{groups.length}</strong></div>
                <div>{t('group.targetCapacityLabel')} <strong>{t('group.studentsUnit', { count: targetGroupCap })}</strong></div>
              </div>
            }
          />

          <div>
            <Text strong>{t('group.selectSolverLabel')}</Text>
            <Radio.Group
              value={selectedSolver}
              onChange={(e) => setSelectedSolver(e.target.value)}
              style={{ width: '100%', marginTop: 8 }}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                <Radio value="optimal">
                  <div>
                    <strong>{t('group.solverOptimalTitle')}</strong>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                      {t('group.solverOptimalDesc')}
                    </Text>
                  </div>
                </Radio>
                <Radio value="regret">
                  <div>
                    <strong>{t('group.solverRegretTitle')}</strong>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                      {t('group.solverRegretDesc')}
                    </Text>
                  </div>
                </Radio>
                <Radio value="greedy">
                  <div>
                    <strong>{t('group.solverGreedyTitle')}</strong>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                      {t('group.solverGreedyDesc')}
                    </Text>
                  </div>
                </Radio>
              </Space>
            </Radio.Group>
          </div>
        </Space>
      </Modal>

      {/* Admin Auto-Assign Results Modal */}
      <Modal
        title={
          <Space>
            <CheckCircleOutlined style={{ color: '#52c41a' }} />
            <span>{t('group.autoAssignComplete')}</span>
          </Space>
        }
        open={resultModalVisible}
        onOk={() => setResultModalVisible(false)}
        onCancel={() => setResultModalVisible(false)}
        okText={t('common.done')}
        cancelButtonProps={{ style: { display: 'none' } }}
      >
        {assignResult && (
          <Space direction="vertical" style={{ width: '100%', marginTop: 8 }}>
            <Alert
              type="success"
              message={t('group.autoAssignSuccessSummary', { count: assignResult.assigned_count })}
            />
            <Divider style={{ margin: '12px 0' }} />
            <Row gutter={16}>
              <Col span={12}>
                <Statistic
                  title={t('group.solverUsed')}
                  value={
                    assignResult.solver_used === 'optimal'
                      ? t('group.solverOptimalShort')
                      : assignResult.solver_used === 'regret'
                      ? t('group.solverRegretShort')
                      : t('group.solverGreedyShort')
                  }
                  valueStyle={{ fontSize: 16 }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('group.studentsAssigned')}
                  value={assignResult.assigned_count}
                />
              </Col>
            </Row>
            <Row gutter={16} style={{ marginTop: 12 }}>
              <Col span={12}>
                <Statistic
                  title={t('group.avgSatisfaction')}
                  value={assignResult.avg_satisfaction}
                  suffix="/ 10"
                  precision={1}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t('group.topChoiceMatch')}
                  value={assignResult.top_choice_pct}
                  suffix="%"
                  precision={1}
                  valueStyle={{ color: '#3f8600' }}
                />
              </Col>
            </Row>
          </Space>
        )}
      </Modal>

      {/* Admin Assign Student Modal */}
      <Modal
        title={t('group.assignStudentTitle')}
        open={assignModalVisible}
        onOk={handleConfirmAssign}
        onCancel={() => setAssignModalVisible(false)}
        confirmLoading={assignLoading}
        okButtonProps={{ disabled: !selectedStudentUserId }}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
      >
        <Space direction="vertical" style={{ width: '100%', marginTop: 12 }}>
          <Text>{t('group.assignStudentPrompt')}</Text>
          <Select
            showSearch
            placeholder={t('group.selectStudentPlaceholder')}
            style={{ width: '100%' }}
            value={selectedStudentUserId}
            onChange={(val) => setSelectedStudentUserId(val)}
            filterOption={(input, option) =>
              String(option?.label ?? '')
                .toLowerCase()
                .includes(input.toLowerCase())
            }
            options={courseStudents.map((s) => ({
              value: s.user.id,
              label: `${s.user.first_name} ${s.user.last_name} (${s.user.student_number || s.user.email})`,
            }))}
          />
        </Space>
      </Modal>
    </Space>
  );
};

export default GroupsPanel;
