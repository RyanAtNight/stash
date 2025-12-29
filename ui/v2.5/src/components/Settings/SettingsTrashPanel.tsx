import React, { useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import { FormattedDate, FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { FileSize } from "../Shared/FileSize";
import { SettingSection } from "./SettingSection";
import { useToast } from "src/hooks/Toast";
import { faTrash, faUndo } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";

export const SettingsTrashPanel: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [libraryFilter, setLibraryFilter] = useState<string>("");

  const { data, loading, refetch } = GQL.useTrashContentsQuery({
    variables: {
      filter: libraryFilter ? { libraryPath: libraryFilter } : undefined,
    },
  });

  const [restoreFromTrash] = GQL.useRestoreFromTrashMutation();
  const [restoreMultiple] = GQL.useRestoreMultipleFromTrashMutation();
  const [permanentlyDelete] = GQL.usePermanentlyDeleteFromTrashMutation();
  const [emptyTrash] = GQL.useEmptyTrashMutation();

  const trashedFiles = data?.trashContents ?? [];

  // Get unique library paths for filter dropdown
  const libraryPaths = [...new Set(trashedFiles.map((f) => f.libraryPath))];

  function toggleSelection(id: string) {
    const newSelection = new Set(selectedIds);
    if (newSelection.has(id)) {
      newSelection.delete(id);
    } else {
      newSelection.add(id);
    }
    setSelectedIds(newSelection);
  }

  function toggleSelectAll() {
    if (selectedIds.size === trashedFiles.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(trashedFiles.map((f) => f.id)));
    }
  }

  async function handleRestore() {
    if (selectedIds.size === 0) return;

    try {
      if (selectedIds.size === 1) {
        const id = [...selectedIds][0];
        const result = await restoreFromTrash({ variables: { id } });
        if (result.data?.restoreFromTrash.success) {
          Toast.success(
            intl.formatMessage(
              { id: "config.trash.restored_file" },
              { path: result.data.restoreFromTrash.restoredPath }
            )
          );
        } else {
          Toast.error(result.data?.restoreFromTrash.error ?? "Restore failed");
        }
      } else {
        const result = await restoreMultiple({
          variables: { ids: [...selectedIds] },
        });
        const data = result.data?.restoreMultipleFromTrash;
        if (data) {
          Toast.success(
            intl.formatMessage(
              { id: "config.trash.restored_multiple" },
              { count: data.restored, failed: data.failed }
            )
          );
        }
      }
      setSelectedIds(new Set());
      refetch();
    } catch (e) {
      Toast.error(e instanceof Error ? e.message : "Restore failed");
    }
  }

  async function handleDelete() {
    if (selectedIds.size === 0) return;

    if (
      !window.confirm(
        intl.formatMessage({ id: "config.trash.confirm_permanent_delete" })
      )
    ) {
      return;
    }

    try {
      await permanentlyDelete({ variables: { ids: [...selectedIds] } });
      Toast.success(intl.formatMessage({ id: "config.trash.deleted_permanently" }));
      setSelectedIds(new Set());
      refetch();
    } catch (e) {
      Toast.error(e instanceof Error ? e.message : "Delete failed");
    }
  }

  async function handleEmptyTrash() {
    if (
      !window.confirm(intl.formatMessage({ id: "config.trash.confirm_empty_trash" }))
    ) {
      return;
    }

    try {
      await emptyTrash({
        variables: { libraryPath: libraryFilter || undefined },
      });
      Toast.success(intl.formatMessage({ id: "config.trash.emptied" }));
      setSelectedIds(new Set());
      refetch();
    } catch (e) {
      Toast.error(e instanceof Error ? e.message : "Empty trash failed");
    }
  }

  if (loading) {
    return <div>Loading...</div>;
  }

  return (
    <>
      <SettingSection headingID="config.trash.heading">
        <div className="content">
          <div className="trash-controls">
            <Form.Group className="d-inline-block mr-3">
              <Form.Label>
                <FormattedMessage id="config.trash.filter_library" />
              </Form.Label>
              <Form.Control
                as="select"
                value={libraryFilter}
                onChange={(e) => setLibraryFilter(e.target.value)}
                className="ml-2 d-inline-block w-auto"
              >
                <option value="">
                  {intl.formatMessage({ id: "config.trash.all_libraries" })}
                </option>
                {libraryPaths.map((path) => (
                  <option key={path} value={path}>
                    {path}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>

            <Button
              variant="danger"
              onClick={handleEmptyTrash}
              disabled={trashedFiles.length === 0}
            >
              <Icon icon={faTrash} />
              <span className="ml-2">
                <FormattedMessage id="config.trash.empty_trash" />
              </span>
            </Button>
          </div>

          {trashedFiles.length === 0 ? (
            <div className="text-muted">
              <FormattedMessage id="config.trash.empty" />
            </div>
          ) : (
            <>
              <div>
                <Button
                  variant="primary"
                  onClick={handleRestore}
                  disabled={selectedIds.size === 0}
                  className="mr-2"
                >
                  <Icon icon={faUndo} />
                  <span className="ml-2">
                    <FormattedMessage id="config.trash.restore_selected" />
                  </span>
                </Button>
                <Button
                  variant="danger"
                  onClick={handleDelete}
                  disabled={selectedIds.size === 0}
                >
                  <Icon icon={faTrash} />
                  <span className="ml-2">
                    <FormattedMessage id="config.trash.delete_selected" />
                  </span>
                </Button>
                {selectedIds.size > 0 && (
                  <span className="ml-3 text-muted">
                    <FormattedMessage
                      id="config.trash.selected_count"
                      values={{ count: selectedIds.size }}
                    />
                  </span>
                )}
              </div>

              <Table responsive striped bordered hover size="sm">
                <thead>
                  <tr>
                    <th style={{ width: "40px" }}>
                      <Form.Check
                        type="checkbox"
                        checked={selectedIds.size === trashedFiles.length}
                        onChange={toggleSelectAll}
                      />
                    </th>
                    <th>
                      <FormattedMessage id="config.trash.filename" />
                    </th>
                    <th>
                      <FormattedMessage id="config.trash.original_location" />
                    </th>
                    <th>
                      <FormattedMessage id="config.trash.deleted_at" />
                    </th>
                    <th>
                      <FormattedMessage id="config.trash.size" />
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {trashedFiles.map((file) => (
                    <tr key={file.id}>
                      <td>
                        <Form.Check
                          type="checkbox"
                          checked={selectedIds.has(file.id)}
                          onChange={() => toggleSelection(file.id)}
                        />
                      </td>
                      <td>{file.fileName}</td>
                      <td>
                        {file.originalPath || (
                          <span className="text-muted">
                            <FormattedMessage id="config.trash.unknown_location" />
                          </span>
                        )}
                      </td>
                      <td>
                        <FormattedDate
                          value={file.deletedAt}
                          year="numeric"
                          month="short"
                          day="numeric"
                          hour="numeric"
                          minute="numeric"
                        />
                      </td>
                      <td>
                        <FileSize size={file.fileSize} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </>
          )}
        </div>
      </SettingSection>
    </>
  );
};
